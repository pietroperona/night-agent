package intercept

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	dylibName      = "guardian-intercept.dylib"
	envDYLD        = "DYLD_INSERT_LIBRARIES"
	envSocket      = "GUARDIAN_SOCKET"
	envBypass      = "GUARDIAN_BYPASS"
)

// BuildEnv costruisce le variabili d'ambiente per il processo agente,
// aggiungendo DYLD_INSERT_LIBRARIES e GUARDIAN_SOCKET.
// Le variabili già presenti in base vengono preservate,
// eccetto quelle che sovrascriviamo esplicitamente.
func BuildEnv(base []string, dylibPath, socketPath string) []string {
	overrides := map[string]string{
		envDYLD:   dylibPath,
		envSocket: socketPath,
	}

	result := make([]string, 0, len(base)+2)
	for _, e := range base {
		key := envKey(e)
		if _, skip := overrides[key]; skip {
			continue
		}
		result = append(result, e)
	}

	for k, v := range overrides {
		result = append(result, k+"="+v)
	}
	return result
}

// FindDylib cerca il file guardian-intercept.dylib in una directory.
func FindDylib(searchDir string) (string, error) {
	candidate := filepath.Join(searchDir, dylibName)
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}
	return "", fmt.Errorf("%s non trovata in %s — esegui 'make' nella root del progetto", dylibName, searchDir)
}

func envKey(entry string) string {
	if idx := strings.IndexByte(entry, '='); idx >= 0 {
		return entry[:idx]
	}
	return entry
}
