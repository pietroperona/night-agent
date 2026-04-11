package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/pietroperona/agent-guardian/internal/sandbox"
	"github.com/spf13/cobra"
)

var sandboxCmd = &cobra.Command{
	Use:   "sandbox",
	Short: "Esegui comandi in ambiente Docker isolato",
}

var sandboxRunCmd = &cobra.Command{
	Use:   "run <comando>",
	Short: "Esegui un comando in sandbox Docker",
	Long: `Esegue il comando specificato all'interno di un container Docker isolato.

Il workspace corrente viene montato nel container.
La rete è disabilitata per default (--network none).

Esempi:
  guardian sandbox run "python migration_script.py"
  guardian sandbox run "bash deploy.sh"
  guardian sandbox run --image alpine:3.20 --network bridge "curl https://example.com"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runSandbox,
}

var (
	sandboxImage   string
	sandboxNetwork string
)

func init() {
	sandboxRunCmd.Flags().StringVar(&sandboxImage, "image", sandbox.DefaultImage,
		"immagine Docker da usare")
	sandboxRunCmd.Flags().StringVar(&sandboxNetwork, "network", sandbox.DefaultNetwork,
		"modalità rete del container: none o bridge")

	sandboxCmd.AddCommand(sandboxRunCmd)
	rootCmd.AddCommand(sandboxCmd)
}

func runSandbox(cmd *cobra.Command, args []string) error {
	mgr := sandbox.New()

	if !mgr.IsAvailable() {
		return fmt.Errorf("Docker non disponibile — installa Docker Desktop e assicurati che il daemon sia in esecuzione")
	}

	// Ricostruisce il comando completo dagli argomenti
	command := strings.Join(args, " ")

	workDir, err := os.Getwd()
	if err != nil {
		workDir = ""
	}

	cfg := sandbox.Config{
		Image:   sandboxImage,
		Network: sandboxNetwork,
		WorkDir: workDir,
	}

	fmt.Printf("\033[33m[⬡ sandbox]\033[0m %s\n", command)
	fmt.Printf("  immagine: %s  rete: %s\n", cfg.Image, cfg.Network)
	if workDir != "" {
		fmt.Printf("  workspace: %s → /workspace\n", workDir)
	}
	fmt.Println()

	result, err := mgr.Execute(context.Background(), command, cfg)
	if err != nil {
		return fmt.Errorf("esecuzione sandbox fallita: %w", err)
	}

	if result.Stdout != "" {
		fmt.Print(result.Stdout)
	}
	if result.Stderr != "" {
		fmt.Fprint(os.Stderr, result.Stderr)
	}

	fmt.Printf("\n\033[33m[⬡ sandbox]\033[0m completato con exit code %d\n", result.ExitCode)

	if result.ExitCode != 0 {
		os.Exit(result.ExitCode)
	}
	return nil
}
