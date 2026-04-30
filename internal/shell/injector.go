package shell

import (
	"fmt"
	"os"
	"strings"
)

const (
	beginMarker = "# BEGIN nightagent"
	endMarker   = "# END nightagent"
)

// hookTemplate è la funzione zsh iniettata nel profilo shell.
// Usa preexec (eseguita prima di ogni comando) per intercettare i comandi
// e inviarli al daemon via socat/nc sul Unix socket.
const hookTemplate = `
# BEGIN nightagent
# Night Agent — hook di intercettazione comandi (non modificare manualmente)
_nightagent_socket="%s"
_nightagent_preexec() {
  local cmd="$1"
  local workdir="$(pwd)"
  if [[ -S "$_nightagent_socket" ]]; then
    local payload="{\"command\":$(printf '%%s' "$cmd" | python3 -c 'import json,sys; print(json.dumps(sys.stdin.read()))'),\"work_dir\":\"$workdir\",\"agent_name\":\"\"}"
    local response
    response=$(echo "$payload" | nc -U "$_nightagent_socket" 2>/dev/null)
    local decision
    decision=$(echo "$response" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d.get("decision","allow"))' 2>/dev/null)
    if [[ "$decision" == "block" ]]; then
      local reason
      reason=$(echo "$response" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d.get("reason",""))' 2>/dev/null)
      echo "nightagent: comando bloccato — $reason" >&2
      return 1
    fi
    if [[ "$decision" == "sandbox" ]]; then
      local reason
      reason=$(echo "$response" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d.get("reason",""))' 2>/dev/null)
      echo "nightagent: esecuzione in sandbox — $reason" >&2
      local output
      output=$(echo "$response" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d.get("output",""), end="")' 2>/dev/null)
      [[ -n "$output" ]] && echo "$output"
      return 1
    fi
  fi
}
autoload -Uz add-zsh-hook
add-zsh-hook preexec _nightagent_preexec
# END nightagent
`

// Inject aggiunge l'hook nightagent al file di profilo shell specificato.
// L'operazione è idempotente: se l'hook è già presente non viene duplicato.
// Restituisce (true, nil) se l'hook è stato iniettato ora,
// (false, nil) se era già presente, (false, err) in caso di errore.
func Inject(rcPath, socketPath string) (bool, error) {
	content, err := os.ReadFile(rcPath)
	if err != nil {
		return false, fmt.Errorf("impossibile leggere %s: %w", rcPath, err)
	}

	if strings.Contains(string(content), beginMarker) {
		return false, nil // già iniettato
	}

	hook := fmt.Sprintf(hookTemplate, socketPath)
	updated := string(content) + hook

	return true, os.WriteFile(rcPath, []byte(updated), 0600)
}

// Remove elimina il blocco nightagent dal file di profilo shell.
func Remove(rcPath string) error {
	content, err := os.ReadFile(rcPath)
	if err != nil {
		return fmt.Errorf("impossibile leggere %s: %w", rcPath, err)
	}

	s := string(content)
	start := strings.Index(s, beginMarker)
	end := strings.Index(s, endMarker)

	if start == -1 || end == -1 {
		return nil // nessun hook da rimuovere
	}

	end += len(endMarker)
	// rimuovi anche l'eventuale newline dopo il marker di chiusura
	if end < len(s) && s[end] == '\n' {
		end++
	}

	updated := s[:start] + s[end:]
	return os.WriteFile(rcPath, []byte(updated), 0600)
}

// IsInjected verifica se l'hook nightagent è già presente nel file.
func IsInjected(rcPath string) bool {
	content, err := os.ReadFile(rcPath)
	if err != nil {
		return false
	}
	return strings.Contains(string(content), beginMarker)
}
