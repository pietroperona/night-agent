package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/pietroperona/night-agent/internal/mcphook"
	"github.com/spf13/cobra"
)

var mcpHookCmd = &cobra.Command{
	Use:   "mcp-hook",
	Short: "Hook PreToolUse per Claude Code — valuta tool call prima dell'esecuzione",
	Long: `Invocato automaticamente da Claude Code via PreToolUse hook.
Legge il contesto della tool call da stdin (JSON), invia al daemon e
restituisce exit code 0 (allow) o 2 (block).

Configurazione in ~/.claude/settings.json:
  {
    "hooks": {
      "PreToolUse": [{
        "matcher": "*",
        "hooks": [{"type": "command", "command": "/path/to/nightagent mcp-hook"}]
      }]
    }
  }`,
	RunE: runMCPHook,
}

func init() {
	rootCmd.AddCommand(mcpHookCmd)
}

func runMCPHook(cmd *cobra.Command, args []string) error {
	parsed, err := mcphook.ParseStdin(os.Stdin)
	if err != nil {
		// fail-open: input malformato non deve bloccare il workflow
		fmt.Fprintf(os.Stderr, "[night-agent] errore parsing input: %v — consento\n", err)
		os.Exit(0)
	}

	req := mcphook.BuildDaemonRequest(parsed)

	home, _ := os.UserHomeDir()
	socketPath := filepath.Join(home, ".night-agent", "night-agent.sock")

	decision, reason := queryDaemon(socketPath, req)

	code := mcphook.ExitCode(decision)
	if code != 0 {
		fmt.Fprintf(os.Stderr, "[guardian] bloccato: %s — %s\n", parsed.Command, reason)
	} else if decision == "sandbox" {
		fmt.Fprintf(os.Stderr, "[guardian] sandbox: %s — %s\n", parsed.Command, reason)
	}

	os.Exit(code)
	return nil
}

func queryDaemon(socketPath string, req mcphook.DaemonRequest) (decision, reason string) {
	conn, err := net.DialTimeout("unix", socketPath, 2*time.Second)
	if err != nil {
		return "allow", ""
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(3 * time.Second))

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return "allow", ""
	}

	var resp struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return "allow", ""
	}

	return resp.Decision, resp.Reason
}
