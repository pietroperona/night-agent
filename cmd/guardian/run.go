package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pietroperona/agent-guardian/internal/intercept"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run <agente> [args...]",
	Short: "Avvia un agente AI sotto la protezione di Guardian",
	Long: `Avvia un agente AI iniettando la libreria di interception.
Ogni comando shell eseguito dall'agente viene intercettato e valutato
dalla policy prima dell'esecuzione.

Esempi:
  guardian run claude
  guardian run python3 my_agent.py
  guardian run node agent.js`,
	Args: cobra.MinimumNArgs(1),
	RunE: runAgent,
}

func init() {
	rootCmd.AddCommand(runCmd)
}

func runAgent(cmd *cobra.Command, args []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	guardianDir := filepath.Join(home, ".guardian")
	socketPath := filepath.Join(guardianDir, "guardian.sock")

	// verifica che il daemon sia in esecuzione
	if !isDaemonRunning(socketPath) {
		return fmt.Errorf("daemon non in esecuzione — avvia prima 'guardian start' in un altro terminale")
	}

	// cerca la dylib nella stessa directory del binario o nella root del progetto
	binaryDir := filepath.Dir(os.Args[0])
	dylibPath, err := findDylibCandidates(binaryDir)
	if err != nil {
		return err
	}

	agentBinary, err := exec.LookPath(args[0])
	if err != nil {
		return fmt.Errorf("agente '%s' non trovato nel PATH: %w", args[0], err)
	}

	env := intercept.BuildEnv(os.Environ(), dylibPath, socketPath)

	fmt.Printf("guardian: avvio '%s' con interception attiva\n", args[0])
	fmt.Printf("guardian: dylib  → %s\n", dylibPath)
	fmt.Printf("guardian: socket → %s\n\n", socketPath)

	agentCmd := exec.Command(agentBinary, args[1:]...)
	agentCmd.Env = env
	agentCmd.Stdin = os.Stdin
	agentCmd.Stdout = os.Stdout
	agentCmd.Stderr = os.Stderr

	return agentCmd.Run()
}

func findDylibCandidates(binaryDir string) (string, error) {
	candidates := []string{
		binaryDir,
		".", // directory corrente (sviluppo locale)
	}
	for _, dir := range candidates {
		if path, err := intercept.FindDylib(dir); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf(
		"guardian-intercept.dylib non trovata — esegui 'make dylib' nella root del progetto",
	)
}
