// Package claudehook gestisce la configurazione del hook PreToolUse
// in ~/.claude/settings.json per l'integrazione con Claude Code.
package claudehook

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const hookMarker = "nightagent mcp-hook"

// IsClaudeInstalled rileva se Claude Code è installato controllando:
// 1. il binario `claude` nel PATH
// 2. l'esistenza di ~/.claude/ (directory di configurazione Claude Code)
func IsClaudeInstalled() bool {
	if _, err := exec.LookPath("claude"); err == nil {
		return true
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(home, ".claude"))
	return err == nil
}

// IsConfigured verifica se il hook nightagent è già in settings.json.
func IsConfigured(settingsPath string) bool {
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return false
	}
	// ricerca veloce stringa — evita parsing JSON completo
	for i := 0; i+len(hookMarker) <= len(data); i++ {
		if string(data[i:i+len(hookMarker)]) == hookMarker {
			return true
		}
	}
	return false
}

// Install aggiunge il hook PreToolUse a ~/.claude/settings.json.
// Crea il file se non esiste. Idempotente — non aggiunge il hook due volte.
// nightagentBin è il path assoluto del binario nightagent da usare nel hook.
func Install(settingsPath, nightagentBin string) error {
	if IsConfigured(settingsPath) {
		return nil // già configurato
	}

	settings, err := loadSettings(settingsPath)
	if err != nil {
		return fmt.Errorf("lettura settings.json: %w", err)
	}

	hookCmd := nightagentBin + " mcp-hook"
	addHook(settings, hookCmd)

	return writeSettings(settingsPath, settings)
}

// Remove rimuove il hook nightagent da ~/.claude/settings.json.
func Remove(settingsPath string) error {
	if !IsConfigured(settingsPath) {
		return nil // non presente
	}

	settings, err := loadSettings(settingsPath)
	if err != nil {
		return fmt.Errorf("lettura settings.json: %w", err)
	}

	removeHook(settings)
	return writeSettings(settingsPath, settings)
}

// SettingsPath restituisce il path standard di ~/.claude/settings.json.
func SettingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

// --- helpers ---

func loadSettings(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]interface{}{}, nil
	}
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func writeSettings(path string, settings map[string]interface{}) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0600)
}

// addHook inserisce il hook PreToolUse nella struttura settings.
func addHook(settings map[string]interface{}, hookCmd string) {
	hooks, _ := settings["hooks"].(map[string]interface{})
	if hooks == nil {
		hooks = map[string]interface{}{}
	}

	newEntry := map[string]interface{}{
		"matcher": "*",
		"hooks": []interface{}{
			map[string]interface{}{
				"type":    "command",
				"command": hookCmd,
			},
		},
	}

	preToolUse, _ := hooks["PreToolUse"].([]interface{})
	hooks["PreToolUse"] = append(preToolUse, newEntry)
	settings["hooks"] = hooks
}

// removeHook rimuove le voci PreToolUse che contengono il marker nightagent.
func removeHook(settings map[string]interface{}) {
	hooks, ok := settings["hooks"].(map[string]interface{})
	if !ok {
		return
	}

	preToolUse, ok := hooks["PreToolUse"].([]interface{})
	if !ok {
		return
	}

	filtered := preToolUse[:0]
	for _, entry := range preToolUse {
		entryMap, ok := entry.(map[string]interface{})
		if !ok {
			filtered = append(filtered, entry)
			continue
		}
		innerHooks, _ := entryMap["hooks"].([]interface{})
		hasMarker := false
		for _, h := range innerHooks {
			hMap, _ := h.(map[string]interface{})
			cmd, _ := hMap["command"].(string)
			if len(cmd) >= len(hookMarker) && cmd[len(cmd)-len(hookMarker):] == hookMarker {
				hasMarker = true
				break
			}
			// controlla anche se la stringa contiene il marker (path assoluto)
			for i := 0; i+len(hookMarker) <= len(cmd); i++ {
				if cmd[i:i+len(hookMarker)] == hookMarker {
					hasMarker = true
					break
				}
			}
		}
		if !hasMarker {
			filtered = append(filtered, entry)
		}
	}

	if len(filtered) == 0 {
		delete(hooks, "PreToolUse")
	} else {
		hooks["PreToolUse"] = filtered
	}

	if len(hooks) == 0 {
		delete(settings, "hooks")
	}
}
