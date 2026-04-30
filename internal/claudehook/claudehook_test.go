package claudehook_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/night-agent-cli/night-agent/internal/claudehook"
)

func TestIsConfigured_FalseWhenMissing(t *testing.T) {
	dir := t.TempDir()
	if claudehook.IsConfigured(filepath.Join(dir, "settings.json")) {
		t.Error("atteso false su file mancante")
	}
}

func TestInstall_CreatesHook(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	if err := claudehook.Install(path, "/usr/local/bin/nightagent"); err != nil {
		t.Fatalf("Install: %v", err)
	}

	if !claudehook.IsConfigured(path) {
		t.Error("hook non trovato dopo Install")
	}

	// verifica struttura JSON
	data, _ := os.ReadFile(path)
	var m map[string]interface{}
	json.Unmarshal(data, &m)

	hooks, _ := m["hooks"].(map[string]interface{})
	if hooks == nil {
		t.Fatal("campo hooks mancante")
	}
	preToolUse, _ := hooks["PreToolUse"].([]interface{})
	if len(preToolUse) == 0 {
		t.Fatal("PreToolUse vuoto")
	}
}

func TestInstall_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	claudehook.Install(path, "/usr/local/bin/nightagent")
	claudehook.Install(path, "/usr/local/bin/nightagent")

	data, _ := os.ReadFile(path)
	var m map[string]interface{}
	json.Unmarshal(data, &m)

	hooks := m["hooks"].(map[string]interface{})
	preToolUse := hooks["PreToolUse"].([]interface{})
	if len(preToolUse) != 1 {
		t.Errorf("attesa 1 entry PreToolUse, trovate %d", len(preToolUse))
	}
}

func TestInstall_PreservesExistingSettings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	existing := `{"env": {}, "permissions": {"allow": ["Bash"]}}`
	os.WriteFile(path, []byte(existing), 0600)

	if err := claudehook.Install(path, "/usr/local/bin/nightagent"); err != nil {
		t.Fatalf("Install: %v", err)
	}

	data, _ := os.ReadFile(path)
	var m map[string]interface{}
	json.Unmarshal(data, &m)

	// campo pre-esistente deve essere preservato
	perms, _ := m["permissions"].(map[string]interface{})
	if perms == nil {
		t.Error("permissions rimosso dopo Install")
	}
}

func TestRemove_RemovesHook(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	claudehook.Install(path, "/usr/local/bin/nightagent")
	if err := claudehook.Remove(path); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if claudehook.IsConfigured(path) {
		t.Error("hook ancora presente dopo Remove")
	}
}

func TestRemove_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	claudehook.Install(path, "/usr/local/bin/nightagent")
	claudehook.Remove(path)
	if err := claudehook.Remove(path); err != nil {
		t.Errorf("secondo Remove ha restituito errore: %v", err)
	}
}
