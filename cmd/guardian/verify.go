package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pietroperona/night-agent/internal/audit"
	"github.com/spf13/cobra"
)

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verifica l'integrità delle firme nell'audit log",
	RunE:  runVerify,
}

func init() {
	rootCmd.AddCommand(verifyCmd)
}

func runVerify(cmd *cobra.Command, args []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	guardianDir := filepath.Join(home, ".night-agent")
	keyPath := filepath.Join(guardianDir, "signing.key")
	logPath := filepath.Join(guardianDir, "audit.jsonl")

	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		return fmt.Errorf("chiave di firma non trovata in %s\nesegui 'nightagent init' per generarla", keyPath)
	}
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		return fmt.Errorf("log non trovato in %s", logPath)
	}

	signer, err := audit.NewSigner(keyPath)
	if err != nil {
		return fmt.Errorf("caricamento chiave: %w", err)
	}

	results, err := audit.VerifyAll(logPath, signer)
	if err != nil {
		return fmt.Errorf("lettura log: %w", err)
	}

	if len(results) == 0 {
		fmt.Println("log vuoto — nessun evento da verificare")
		return nil
	}

	var invalid, unsigned, valid int
	for _, r := range results {
		switch {
		case r.Err == nil:
			valid++
		case r.EventID == "":
			invalid++
			fmt.Fprintf(os.Stderr, "  [✗] evento #%d: %v\n", r.Index+1, r.Err)
		default:
			if r.Err.Error() == "evento senza firma (sig assente)" {
				unsigned++
			} else {
				invalid++
				fmt.Fprintf(os.Stderr, "  [✗] evento %s (#%d): %v\n", r.EventID, r.Index+1, r.Err)
			}
		}
	}

	fmt.Printf("audit log: %d eventi totali\n", len(results))
	fmt.Printf("  ✓ validi:    %d\n", valid)
	if unsigned > 0 {
		fmt.Printf("  · non firmati: %d  (eventi precedenti all'attivazione della firma)\n", unsigned)
	}
	if invalid > 0 {
		fmt.Printf("  ✗ manomessi: %d\n", invalid)
		return fmt.Errorf("%d eventi con firma non valida — log potenzialmente manomesso", invalid)
	}

	fmt.Println("\nintegrità verificata.")
	return nil
}
