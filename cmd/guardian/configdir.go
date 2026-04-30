package main

import (
	"os"

	"github.com/night-agent-cli/night-agent/internal/configdir"
)

// resolveConfigDir restituisce la config dir da usare per il comando corrente.
// Se --global è passato → ~/.night-agent/.
// Altrimenti → .nightagent/ nella cwd se esiste, fallback ~/.night-agent/.
func resolveConfigDir() (string, error) {
	if globalConfig {
		return configdir.Global()
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return configdir.Resolve(cwd)
}
