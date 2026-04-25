package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/night-agent-cli/night-agent/internal/audit"
	"github.com/night-agent-cli/night-agent/internal/cloudconfig"
	"github.com/night-agent-cli/night-agent/internal/configdir"
	cloudsync "github.com/night-agent-cli/night-agent/internal/sync"
	"github.com/spf13/cobra"
)

var cloudCmd = &cobra.Command{
	Use:   "cloud",
	Short: "Gestisci connessione cloud Night Agent",
}

var cloudConnectCmd = &cobra.Command{
	Use:   "connect <TOKEN>",
	Short: "Connetti al cloud Night Agent con il token fornito",
	Args:  cobra.ExactArgs(1),
	RunE:  runCloudConnect,
}

var cloudStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Mostra stato connessione cloud",
	RunE:  runCloudStatus,
}

var cloudDisconnectCmd = &cobra.Command{
	Use:   "disconnect",
	Short: "Disconnetti dal cloud Night Agent",
	RunE:  runCloudDisconnect,
}

var cloudSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sincronizza manualmente gli eventi con il cloud",
	RunE:  runCloudSync,
}

func init() {
	cloudCmd.AddCommand(cloudConnectCmd)
	cloudCmd.AddCommand(cloudStatusCmd)
	cloudCmd.AddCommand(cloudDisconnectCmd)
	cloudCmd.AddCommand(cloudSyncCmd)
	rootCmd.AddCommand(cloudCmd)
}

// cloudConfigPath restituisce il path di cloud.yaml nella config dir risolta.
func cloudConfigPath() (string, error) {
	dir, err := resolveConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "cloud.yaml"), nil
}

// cloudLogPath restituisce il path di audit.jsonl nella config dir risolta.
func cloudLogPath() (string, error) {
	dir, err := resolveConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "audit.jsonl"), nil
}

func runCloudConnect(_ *cobra.Command, args []string) error {
	token := args[0]

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	// connect crea sempre una config dir locale (a meno che --global)
	var cfgDir string
	if globalConfig {
		cfgDir, err = configdir.Global()
	} else {
		cfgDir, err = configdir.CreateLocal(cwd)
	}
	if err != nil {
		return err
	}

	cfgPath := filepath.Join(cfgDir, "cloud.yaml")

	// genera signing key dedicata per questa config dir
	keyPath := filepath.Join(cfgDir, "signing.key")
	if err := audit.GenerateKey(keyPath); err != nil {
		return fmt.Errorf("generazione signing key: %w", err)
	}

	cfg, err := cloudconfig.Connect(cfgPath, token)
	if err != nil {
		return fmt.Errorf("connessione fallita: %w", err)
	}

	// aggiungi .nightagent/ al .gitignore del progetto se non è config globale
	if configdir.IsLocal(cfgDir) {
		if err := addToGitignore(cwd, configdir.LocalDirName+"/"); err != nil {
			fmt.Fprintf(os.Stderr, "  avviso: impossibile aggiornare .gitignore: %v\n", err)
		}
	}

	if err := registerSigningKey(cfg, keyPath); err != nil {
		fmt.Fprintf(os.Stderr, "  avviso: registrazione chiave di firma fallita: %v\n", err)
	}

	fmt.Println("  ✓ connesso al cloud Night Agent")
	fmt.Printf("  config dir : %s\n", cfgDir)
	fmt.Printf("  endpoint   : %s\n", cfg.Endpoint)
	fmt.Printf("  machine    : %s\n", cfg.MachineID)
	fmt.Println()
	fmt.Println("  sync automatico: avvia il daemon con 'nightagent start'")
	fmt.Println("  sync manuale   : 'nightagent cloud sync'")
	return nil
}

// addToGitignore aggiunge entry al .gitignore nella dir indicata se esiste.
// Idempotente: non aggiunge se entry è già presente.
func addToGitignore(dir, entry string) error {
	gitignorePath := filepath.Join(dir, ".gitignore")
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		return nil // nessun .gitignore, niente da fare
	}

	// controlla se entry già presente
	f, err := os.Open(gitignorePath)
	if err != nil {
		return err
	}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == entry {
			f.Close()
			return nil // già presente
		}
	}
	f.Close()

	// appendi entry
	out, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = fmt.Fprintf(out, "\n# Night Agent config locale\n%s\n", entry)
	return err
}

// registerSigningKey invia la chiave di firma al backend cloud.
// La chiave è in formato hex — viene letta e inviata as-is.
func registerSigningKey(cfg *cloudconfig.Config, keyPath string) error {
	keyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("lettura signing.key: %w", err)
	}

	body, err := json.Marshal(map[string]string{
		"machine_id":  cfg.MachineID,
		"signing_key": strings.TrimSpace(string(keyBytes)),
	})
	if err != nil {
		return err
	}

	url := cfg.Endpoint + "/api/machines/signing-key"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server ha risposto %d", resp.StatusCode)
	}
	return nil
}

func runCloudStatus(_ *cobra.Command, _ []string) error {
	dir, err := resolveConfigDir()
	if err != nil {
		return err
	}
	cfgPath := filepath.Join(dir, "cloud.yaml")

	cfg, err := cloudconfig.Load(cfgPath)
	if err != nil {
		return err
	}

	if !cfg.Connected || cfg.Token == "" {
		fmt.Println("  cloud: non connesso")
		fmt.Println("  usa 'nightagent cloud connect <TOKEN>' per connetterti")
		return nil
	}

	fmt.Println("  cloud: connesso")
	fmt.Printf("  config dir : %s\n", dir)
	fmt.Printf("  endpoint   : %s\n", cfg.Endpoint)
	fmt.Printf("  machine    : %s\n", cfg.MachineID)

	if cfg.Cursor != "" {
		fmt.Printf("  cursore    : %s\n", cfg.Cursor)
	} else {
		fmt.Println("  cursore    : nessun sync effettuato")
	}

	if !cfg.LastSync.IsZero() {
		ago := time.Since(cfg.LastSync).Round(time.Second)
		fmt.Printf("  ultimo sync: %s fa (%s)\n", ago, cfg.LastSync.Format("2006-01-02 15:04:05"))
	} else {
		fmt.Println("  ultimo sync: mai")
	}
	return nil
}

func runCloudDisconnect(_ *cobra.Command, _ []string) error {
	dir, err := resolveConfigDir()
	if err != nil {
		return err
	}
	cfgPath := filepath.Join(dir, "cloud.yaml")

	if err := cloudconfig.Disconnect(cfgPath); err != nil {
		return fmt.Errorf("disconnessione fallita: %w", err)
	}

	fmt.Println("  ✓ disconnesso dal cloud Night Agent")
	fmt.Printf("  token rimosso da %s\n", cfgPath)
	return nil
}

func runCloudSync(_ *cobra.Command, _ []string) error {
	dir, err := resolveConfigDir()
	if err != nil {
		return err
	}
	cfgPath := filepath.Join(dir, "cloud.yaml")
	logPath := filepath.Join(dir, "audit.jsonl")

	cfg, err := cloudconfig.Load(cfgPath)
	if err != nil {
		return err
	}
	if !cfg.Connected || cfg.Token == "" {
		return fmt.Errorf("non connesso — esegui 'nightagent cloud connect <TOKEN>'")
	}

	fmt.Print("  sincronizzazione in corso... ")
	agent := cloudsync.NewAgent(cfgPath, logPath)
	if err := agent.SyncOnce(); err != nil {
		fmt.Println("✗")
		return err
	}

	updated, _ := cloudconfig.Load(cfgPath)
	fmt.Println("✓")
	if updated != nil && updated.Cursor != "" {
		fmt.Printf("  ultimo evento sincronizzato: %s\n", updated.Cursor)
	}
	return nil
}
