package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var globalConfig bool

var rootCmd = &cobra.Command{
	Use:   "nightagent",
	Short: "Night Agent — runtime security layer for AI agents",
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&globalConfig, "global", false, "usa ~/.night-agent/ ignorando la config locale del progetto")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
