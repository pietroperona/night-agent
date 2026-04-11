//go:build integration

package intercept_test

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/pietroperona/night-agent/internal/audit"
	"github.com/pietroperona/night-agent/internal/daemon"
	"github.com/pietroperona/night-agent/internal/intercept"
	"github.com/pietroperona/night-agent/internal/policy"
)

// TestDYLD_BlocksCommandViaLibrary è un integration test end-to-end.
//
// Usa exec-helper — un binario non-SIP che chiama exec.Command internamente,
// simulando esattamente come un agente AI (claude-code, node, python) esegue comandi.
// DYLD_INSERT_LIBRARIES non funziona su /bin/sh e /bin/bash (SIP-protetti),
// ma funziona su binari installati dall'utente come claude-code o questo helper.
func TestDYLD_BlocksCommandViaLibrary(t *testing.T) {
	dylibPath := findTestDylib(t)
	helperPath := buildExecHelper(t)
	socketPath, logPath := startIntegrationDaemon(t)

	// exec-helper eseguirà "sudo echo SHOULD_NOT_PRINT" via exec.Command.
	// La dylib intercetta execve() di sudo prima che venga creato il processo.
	cmd := exec.Command(helperPath, "sudo", "echo", "SHOULD_NOT_PRINT")
	cmd.Env = intercept.BuildEnv(os.Environ(), dylibPath, socketPath)

	output, _ := cmd.CombinedOutput()
	out := string(output)

	if containsString(out, "SHOULD_NOT_PRINT") {
		t.Errorf("comando bloccato è stato eseguito lo stesso: %s", out)
	}
	if !containsString(out, "guardian") {
		t.Errorf("atteso messaggio guardian nell'output, ottenuto: %s", out)
	}

	time.Sleep(30 * time.Millisecond)
	events, err := audit.ReadAll(logPath)
	if err != nil {
		t.Fatalf("errore lettura log: %v", err)
	}
	if len(events) == 0 {
		t.Error("atteso almeno 1 evento nel log dopo il blocco")
	}
}

func TestDYLD_AllowsNonBlockedCommand(t *testing.T) {
	dylibPath := findTestDylib(t)
	helperPath := buildExecHelper(t)
	socketPath, _ := startIntegrationDaemon(t)

	cmd := exec.Command(helperPath, "echo", "ALLOWED_OUTPUT")
	cmd.Env = intercept.BuildEnv(os.Environ(), dylibPath, socketPath)

	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("comando permesso ha fallito: %v\noutput: %s", err, output)
	}
	if !containsString(string(output), "ALLOWED_OUTPUT") {
		t.Errorf("atteso ALLOWED_OUTPUT, ottenuto: %s", output)
	}
}

func TestDYLD_SafeFailure_DaemonDown(t *testing.T) {
	dylibPath := findTestDylib(t)
	helperPath := buildExecHelper(t)

	// socket inesistente — daemon non in ascolto: safe failure = blocca
	cmd := exec.Command(helperPath, "echo", "SHOULD_BE_BLOCKED")
	cmd.Env = intercept.BuildEnv(os.Environ(), dylibPath, "/tmp/nonexistent-night-agent.sock")

	output, _ := cmd.CombinedOutput()
	if containsString(string(output), "SHOULD_BE_BLOCKED") {
		t.Error("con daemon down, il comando non dovrebbe essere eseguito (safe failure)")
	}
}

/* ---------- helpers -------------------------------------------------- */

// buildExecHelper compila il C helper e ne restituisce il path.
// Usa C (non Go) perché Go bypassa libc con syscall raw,
// mentre C usa execvp() via libc — esattamente come Node.js/Python/Ruby.
func buildExecHelper(t *testing.T) string {
	t.Helper()
	src := "testdata/exec-helper/main.c"
	out := filepath.Join(t.TempDir(), "exec-helper")
	cmd := exec.Command("clang", "-o", out, src, "-Wall")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("errore compilazione exec-helper C: %v\n%s", err, output)
	}
	return out
}

func findTestDylib(t *testing.T) string {
	t.Helper()
	// cerca nella root del progetto (dove make dylib lo produce)
	candidates := []string{
		"../../guardian-intercept.dylib",
		"guardian-intercept.dylib",
	}
	for _, p := range candidates {
		abs, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		if _, err := os.Stat(abs); err == nil {
			return abs
		}
	}
	t.Skip("guardian-intercept.dylib non trovata — esegui 'make dylib' prima degli integration test")
	return ""
}

func startIntegrationDaemon(t *testing.T) (socketPath, logPath string) {
	t.Helper()

	socketPath = fmt.Sprintf("/tmp/grd-integ-%d.sock", os.Getpid())
	logDir := t.TempDir()
	logPath = filepath.Join(logDir, "audit.jsonl")

	p := &policy.Policy{
		Version: 1,
		Rules: []policy.Rule{
			{
				ID:        "block_sudo",
				MatchType: policy.MatchGlob,
				When:      policy.Condition{ActionType: "shell", CommandMatches: []string{"sudo *", "*/sudo *", "*sudo *"}},
				Decision:  policy.DecisionBlock,
				Reason:    "sudo disabilitato",
			},
		},
	}

	logger, err := audit.NewLogger(logPath)
	if err != nil {
		t.Fatalf("errore logger: %v", err)
	}

	srv, err := daemon.NewServer(socketPath, p, logger)
	if err != nil {
		t.Fatalf("errore daemon: %v", err)
	}

	go srv.Serve()
	t.Cleanup(func() {
		srv.Stop()
		logger.Close()
		os.Remove(socketPath)
	})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if conn, err := net.Dial("unix", socketPath); err == nil {
			conn.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	return socketPath, logPath
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(substr) == 0 ||
		findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// verifica che il JSON del daemon sia valido (smoke test)
func TestDaemonResponseIsValidJSON(t *testing.T) {
	socketPath, _ := startIntegrationDaemon(t)

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("connessione fallita: %v", err)
	}
	defer conn.Close()

	payload := `{"command":"ls -la","work_dir":"/tmp","agent_name":"test"}` + "\n"
	if _, err := conn.Write([]byte(payload)); err != nil {
		t.Fatalf("errore invio: %v", err)
	}

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("errore lettura: %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal(buf[:n], &resp); err != nil {
		t.Errorf("risposta daemon non è JSON valido: %v\nRisposta: %s", err, buf[:n])
	}
}
