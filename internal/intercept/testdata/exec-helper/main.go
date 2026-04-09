// exec-helper è un binario non-SIP usato negli integration test di guardian.
// Riceve un comando come argomento e lo esegue via exec.Command,
// simulando il comportamento di un agente AI (es. claude-code).
package main

import (
	"fmt"
	"os"
	"os/exec"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: exec-helper <cmd> [args...]")
		os.Exit(1)
	}
	cmd := exec.Command(os.Args[1], os.Args[2:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
}
