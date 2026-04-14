package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/pietroperona/night-agent/internal/audit"
	"github.com/pietroperona/night-agent/internal/cloudconfig"
	"github.com/pietroperona/night-agent/internal/configdir"
	"github.com/pietroperona/night-agent/internal/daemon"
	"github.com/pietroperona/night-agent/internal/policy"
	nightsync "github.com/pietroperona/night-agent/internal/sync"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Avvia il daemon Guardian in foreground",
	RunE:  runStart,
}

func init() {
	rootCmd.AddCommand(startCmd)
}

func runStart(cmd *cobra.Command, args []string) error {
	cfgDir, err := resolveConfigDir()
	if err != nil {
		return err
	}

	policyPath := filepath.Join(cfgDir, "policy.yaml")
	socketPath := filepath.Join(cfgDir, "night-agent.sock")
	logPath := filepath.Join(cfgDir, "audit.jsonl")
	cloudCfgPath := filepath.Join(cfgDir, "cloud.yaml")

	// --- Priorità caricamento policy ---
	// 1. cloud configurato → prova GET /api/policy?machine_id=X
	// 2. altrimenti → config dir locale/globale
	// 3. fallback → ~/.night-agent/policy.yaml
	// 4. niente → errore
	resolvedPolicyPath, err := resolvePolicyPath(cloudCfgPath, policyPath)
	if err != nil {
		return err
	}

	p, err := policy.Load(resolvedPolicyPath)
	if err != nil {
		return fmt.Errorf("errore caricamento policy: %w", err)
	}

	// usa signed logger se la chiave esiste, altrimenti logger base
	keyPath := filepath.Join(cfgDir, "signing.key")
	var logger *audit.Logger
	if signer, sigErr := audit.NewSigner(keyPath); sigErr == nil {
		logger, err = audit.NewSignedLogger(logPath, signer)
	} else {
		logger, err = audit.NewLogger(logPath)
	}
	if err != nil {
		return fmt.Errorf("errore apertura log: %w", err)
	}
	defer logger.Close()

	srv, err := daemon.NewServerWithPolicyPath(socketPath, resolvedPolicyPath, p, logger)
	if err != nil {
		return fmt.Errorf("errore avvio daemon: %w", err)
	}
	srv.WithLogPath(logPath)

	fmt.Printf("night-agent in ascolto su %s\n", socketPath)
	fmt.Printf("night-agent policy       : %s\n", resolvedPolicyPath)

	go srv.Serve()

	// sync cloud periodico ogni 30s — fail-open, errori ignorati
	if _, err := os.Stat(cloudCfgPath); err == nil {
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			agent := nightsync.NewAgent(cloudCfgPath, logPath)
			for range ticker.C {
				_ = agent.SyncOnce()
			}
		}()
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("\nnight-agent fermato")
	srv.Stop()
	return nil
}

// resolvePolicyPath implementa la priorità di caricamento policy:
// 1. cloud configurato → GET /api/policy?machine_id=X → scrivi e usa
// 2. policyPath locale/globale se esiste
// 3. fallback ~/.night-agent/policy.yaml
// 4. errore
func resolvePolicyPath(cloudCfgPath, policyPath string) (string, error) {
	// 1. prova cloud
	if cloudPolicy, err := fetchCloudPolicy(cloudCfgPath, policyPath); err == nil && cloudPolicy != "" {
		return cloudPolicy, nil
	}

	// 2. policy nella config dir corrente
	if _, err := os.Stat(policyPath); err == nil {
		return policyPath, nil
	}

	// 3. fallback globale
	globalDir, err := configdir.Global()
	if err == nil {
		globalPolicy := filepath.Join(globalDir, "policy.yaml")
		if _, err := os.Stat(globalPolicy); err == nil {
			return globalPolicy, nil
		}
	}

	// 4. niente trovato
	return "", fmt.Errorf("policy non trovata — esegui 'nightagent init'")
}

// fetchCloudPolicy scarica la policy dal cloud se configurato.
// Se la risposta è non-nulla la scrive su policyPath e restituisce il path.
// Restituisce errore (silenzioso) se non configurato o la chiamata fallisce.
func fetchCloudPolicy(cloudCfgPath, policyPath string) (string, error) {
	cfg, err := cloudconfig.Load(cloudCfgPath)
	if err != nil || !cfg.Connected || cfg.Token == "" {
		return "", fmt.Errorf("cloud non configurato")
	}

	url := cfg.Endpoint + "/api/policy?machine_id=" + cfg.MachineID
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("cloud policy: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if len(body) == 0 {
		return "", fmt.Errorf("cloud policy: risposta vuota")
	}

	// scrivi policy su disco e usala
	if err := os.MkdirAll(filepath.Dir(policyPath), 0700); err != nil {
		return "", err
	}
	if err := os.WriteFile(policyPath, body, 0600); err != nil {
		return "", err
	}

	fmt.Printf("night-agent policy cloud  : scaricata da %s\n", cfg.Endpoint)
	return policyPath, nil
}
