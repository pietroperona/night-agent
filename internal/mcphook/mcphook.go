// Package mcphook implementa il bridge tra i Claude Code hooks (PreToolUse)
// e il daemon di Night Agent. Quando Claude Code invoca nightagent mcp-hook,
// questo package normalizza la tool call MCP in una richiesta daemon standard
// e restituisce l'exit code che Claude Code interpreta come allow/block.
//
// Integrazione Claude Code (~/.claude/settings.json):
//
//	{
//	  "hooks": {
//	    "PreToolUse": [{"matcher": "*", "hooks": [{"type": "command", "command": "nightagent mcp-hook --tool $TOOL_NAME --input-file $TOOL_INPUT_FILE"}]}]
//	  }
//	}
//
// Exit codes: 0 = allow, 2 = block (Claude Code interrompe l'esecuzione).
package mcphook

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ParsedCall è la rappresentazione normalizzata di una MCP tool call.
type ParsedCall struct {
	ToolName  string
	Command   string // comando shell (per Bash) o descrizione (per altri tool)
	Path      string // file path (per Edit, Write, Read, Glob)
	WorkDir   string
	AgentName string
	RawInput  string
}

// DaemonRequest è la struttura inviata al daemon via Unix socket.
// Rispecchia daemon.Request per evitare dipendenza circolare.
type DaemonRequest struct {
	Command   string `json:"command"`
	WorkDir   string `json:"work_dir"`
	AgentName string `json:"agent_name"`
}

// ParseInput analizza il JSON di input di una tool call MCP e restituisce
// una ParsedCall normalizzata. Non fallisce su tool sconosciuti — li passa
// attraverso con il nome del tool come comando (fail-open per tool non rischiosi).
func ParseInput(toolName, inputJSON string) (ParsedCall, error) {
	parsed := ParsedCall{
		ToolName:  toolName,
		RawInput:  inputJSON,
		AgentName: "claude-code",
	}

	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(inputJSON), &raw); err != nil {
		// input non JSON — tratta il tool name come comando generico
		parsed.Command = toolName
		return parsed, nil
	}

	switch toolName {
	case "Bash":
		parsed.Command = stringField(raw, "command")
		parsed.WorkDir = stringField(raw, "workdir")

	case "Edit":
		path := stringField(raw, "file_path")
		parsed.Path = path
		parsed.Command = fmt.Sprintf("edit %s", path)

	case "Write":
		path := stringField(raw, "file_path")
		parsed.Path = path
		parsed.Command = fmt.Sprintf("write %s", path)

	case "Read":
		path := stringField(raw, "file_path")
		parsed.Path = path
		parsed.Command = fmt.Sprintf("read %s", path)

	case "Glob":
		pattern := stringField(raw, "pattern")
		parsed.Command = fmt.Sprintf("glob %s", pattern)

	case "Grep":
		pattern := stringField(raw, "pattern")
		path := stringField(raw, "path")
		parsed.Command = fmt.Sprintf("grep %s %s", pattern, path)
		parsed.Path = path

	case "WebFetch", "WebSearch":
		url := stringField(raw, "url")
		if url == "" {
			url = stringField(raw, "query")
		}
		parsed.Command = fmt.Sprintf("%s %s", strings.ToLower(toolName), url)

	default:
		// tool non mappato — costruisce un comando descrittivo
		parsed.Command = fmt.Sprintf("mcp:%s", toolName)
	}

	return parsed, nil
}

// BuildDaemonRequest costruisce la richiesta da inviare al daemon.
func BuildDaemonRequest(p ParsedCall) DaemonRequest {
	cmd := p.Command
	if cmd == "" {
		cmd = fmt.Sprintf("mcp:%s", p.ToolName)
	}
	return DaemonRequest{
		Command:   cmd,
		WorkDir:   p.WorkDir,
		AgentName: p.AgentName,
	}
}

// ExitCode converte la decisione del daemon nell'exit code da restituire
// a Claude Code. Claude Code interrompe l'esecuzione se exit code != 0.
//
//	allow   → 0  (procedi)
//	sandbox → 0  (eseguito in isolamento, Claude Code non deve intervenire)
//	ask     → 2  (block a runtime — l'utente non è disponibile a rispondere)
//	block   → 2  (blocca)
func ExitCode(decision string) int {
	switch decision {
	case "allow", "sandbox":
		return 0
	default:
		return 2
	}
}

func stringField(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
