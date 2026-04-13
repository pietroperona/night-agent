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
Invia la tool call al daemon e restituisce exit code 0 (allow) o 2 (block).

Configurazione in ~/.claude/settings.json:
  {
    "hooks": {
      "PreToolUse": [{
        "matcher": "*",
        "hooks": [{"type": "command", "command": "nightagent mcp-hook --tool $TOOL_NAME --input-file $TOOL_INPUT_FILE"}]
      }]
    }
  }`,
	RunE: runMCPHook,
}

var (
	flagToolName  string
	flagInputFile string
	flagInputJSON string
)

func init() {
	mcpHookCmd.Flags().StringVar(&flagToolName, "tool", "", "nome del tool MCP (es. Bash, Edit, Write)")
	mcpHookCmd.Flags().StringVar(&flagInputFile, "input-file", "", "path al file JSON con l'input del tool")
	mcpHookCmd.Flags().StringVar(&flagInputJSON, "input", "", "JSON input del tool (alternativa a --input-file)")
	mcpHookCmd.MarkFlagRequired("tool")
	rootCmd.AddCommand(mcpHookCmd)
}

func runMCPHook(cmd *cobra.Command, args []string) error {
	// leggi input JSON
	var inputJSON string
	switch {
	case flagInputFile != "":
		data, err := os.ReadFile(flagInputFile)
		if err != nil {
			return fmt.Errorf("lettura input file: %w", err)
		}
		inputJSON = string(data)
	case flagInputJSON != "":
		inputJSON = flagInputJSON
	default:
		inputJSON = "{}"
	}

	parsed, err := mcphook.ParseInput(flagToolName, inputJSON)
	if err != nil {
		// fail-safe: non bloccare su errore di parsing
		fmt.Fprintf(os.Stderr, "[night-agent] errore parsing input: %v — consento\n", err)
		os.Exit(0)
	}

	req := mcphook.BuildDaemonRequest(parsed)

	// connetti al daemon
	home, _ := os.UserHomeDir()
	socketPath := filepath.Join(home, ".night-agent", "night-agent.sock")

	decision, reason := queryDaemon(socketPath, req)

	code := mcphook.ExitCode(decision)
	if code != 0 {
		fmt.Fprintf(os.Stderr, "[night-agent] %s bloccato — %s\n", flagToolName, reason)
	} else if decision == "sandbox" {
		fmt.Fprintf(os.Stderr, "[night-agent] %s eseguito in sandbox — %s\n", flagToolName, reason)
	}

	os.Exit(code)
	return nil
}

// queryDaemon invia la richiesta al daemon e restituisce decisione e motivo.
// In caso di errore (daemon non in ascolto) fa fail-open: consente l'esecuzione.
func queryDaemon(socketPath string, req mcphook.DaemonRequest) (decision, reason string) {
	conn, err := net.DialTimeout("unix", socketPath, 2*time.Second)
	if err != nil {
		// daemon non disponibile — fail-open (non bloccare il workflow)
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
