package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pietroperona/night-agent/internal/claudehook"
	"github.com/pietroperona/night-agent/internal/launchagent"
	"github.com/pietroperona/night-agent/internal/shell"
	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Rimuove Night Agent dal sistema",
	Long:  "Ferma il daemon, rimuove il LaunchAgent e rimuove l'hook dallo shell profile.",
	RunE:  runUninstall,
}

func init() {
	rootCmd.AddCommand(uninstallCmd)
}

func runUninstall(cmd *cobra.Command, args []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	// rimuovi LaunchAgent
	if launchagent.IsInstalled(home) {
		if err := launchagent.Uninstall(home); err != nil {
			fmt.Printf("avviso: errore rimozione LaunchAgent: %v\n", err)
		} else {
			fmt.Println("LaunchAgent rimosso")
		}
	}

	// rimuovi hook dallo shell profile
	rcPath := filepath.Join(home, ".zshrc")
	if shell.IsInjected(rcPath) {
		if err := shell.Remove(rcPath); err != nil {
			fmt.Printf("avviso: errore rimozione hook da %s: %v\n", rcPath, err)
		} else {
			fmt.Printf("hook rimosso da: %s\n", rcPath)
		}
	}

	// rimuovi hook Claude Code se presente
	if settingsPath, err := claudehook.SettingsPath(); err == nil {
		if claudehook.IsConfigured(settingsPath) {
			if err := claudehook.Remove(settingsPath); err != nil {
				fmt.Printf("avviso: errore rimozione hook Claude Code: %v\n", err)
			} else {
				fmt.Printf("hook Claude Code rimosso da: %s\n", settingsPath)
			}
		}
	}

	fmt.Println("\nnight-agent disinstallato.")
	fmt.Printf("I dati in ~/.night-agent/ sono stati preservati. Per rimuoverli: rm -rf ~/.night-agent\n")
	return nil
}
