package interception

import (
	"fmt"
	"strings"

	"github.com/night-agent-cli/night-agent/internal/policy"
)

// NormalizedAction è il risultato della normalizzazione di un comando raw.
type NormalizedAction struct {
	Type      policy.ActionType
	Command   string
	Path      string
	WorkDir   string
	AgentName string
	IsForce   bool
}

// ToPolicyAction converte NormalizedAction in policy.Action per la valutazione.
func (a NormalizedAction) ToPolicyAction() policy.Action {
	return policy.Action{
		Type:    string(a.Type),
		Command: a.Command,
		Path:    a.Path,
	}
}

// Normalize analizza un comando raw e produce un NormalizedAction classificato.
func Normalize(raw, workDir, agentName string) (NormalizedAction, error) {
	cmd := strings.TrimSpace(raw)
	if cmd == "" {
		return NormalizedAction{}, fmt.Errorf("comando vuoto")
	}

	action := NormalizedAction{
		Command:   cmd,
		WorkDir:   workDir,
		AgentName: agentName,
	}

	action.Type = classifyCommand(cmd)

	if action.Type == policy.ActionTypeGit {
		action.IsForce = isForceGitPush(cmd)
	}

	if action.Type == policy.ActionTypeFile {
		action.Path = extractFilePath(cmd)
	}

	return action, nil
}

// classifyCommand determina il tipo di azione in base al prefisso del comando.
func classifyCommand(cmd string) policy.ActionType {
	if strings.HasPrefix(cmd, "git ") {
		return policy.ActionTypeGit
	}
	if isFileOperation(cmd) {
		return policy.ActionTypeFile
	}
	return policy.ActionTypeShell
}

// isFileOperation rileva redirect di scrittura verso file sensibili o operazioni file esplicite.
func isFileOperation(cmd string) bool {
	fileOps := []string{"cp ", "mv ", "touch ", "chmod ", "chown "}
	for _, op := range fileOps {
		if strings.HasPrefix(cmd, op) {
			return true
		}
	}
	// redirect di scrittura: cmd > file o cmd >> file
	return strings.Contains(cmd, "> ~/") || strings.Contains(cmd, "> /")
}

// isForceGitPush rileva flag di force push.
func isForceGitPush(cmd string) bool {
	return strings.Contains(cmd, "--force") || strings.Contains(cmd, " -f ")
}

// extractFilePath tenta di estrarre il percorso target da un comando file.
func extractFilePath(cmd string) string {
	// per redirect: "cat > /path" → "/path"
	if idx := strings.Index(cmd, "> "); idx != -1 {
		return strings.TrimSpace(cmd[idx+2:])
	}
	// per comandi file: prendi l'ultimo token
	parts := strings.Fields(cmd)
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return ""
}
