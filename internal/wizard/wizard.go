package wizard

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Question rappresenta una domanda del wizard di configurazione.
type Question struct {
	Label        string // identificatore interno
	RuleID       string // ID della regola nella policy
	Description  string // testo mostrato all'utente
	DefaultBlock bool   // true = default blocca, false = default consenti
}

// Prompt restituisce la stringa da mostrare all'utente per questa domanda.
func (q Question) Prompt() string {
	var hint string
	if q.DefaultBlock {
		hint = "[S/n]"
	} else {
		hint = "[s/N]"
	}
	return fmt.Sprintf("  Bloccare: %s? %s ", q.Description, hint)
}

// ParseAnswer interpreta la risposta dell'utente.
// Accetta y/Y/s/S/si/yes come "blocca", n/N/no come "non bloccare".
// Stringa vuota → usa il default.
func ParseAnswer(input string, defaultBlock bool) bool {
	input = strings.TrimSpace(strings.ToLower(input))
	switch input {
	case "y", "s", "si", "yes":
		return true
	case "n", "no":
		return false
	default:
		return defaultBlock
	}
}

// DefaultQuestions restituisce le domande standard del wizard.
func DefaultQuestions() []Question {
	return []Question{
		{
			Label:        "sudo",
			RuleID:       "block_sudo",
			Description:  "sudo (escalation privilegi)",
			DefaultBlock: true,
		},
		{
			Label:        "rm_rf",
			RuleID:       "block_rm_rf",
			Description:  "rm -rf (cancellazione ricorsiva)",
			DefaultBlock: true,
		},
		{
			Label:        "curl_pipe",
			RuleID:       "block_curl_pipe",
			Description:  "curl/wget con pipe (esecuzione script remoti)",
			DefaultBlock: true,
		},
		{
			Label:        "sensitive_paths",
			RuleID:       "block_sensitive_paths",
			Description:  "accesso a file sensibili (~/.ssh, ~/.aws, .env)",
			DefaultBlock: true,
		},
		{
			Label:        "git_push_main",
			RuleID:       "ask_git_push_main",
			Description:  "git push su main/master o --force",
			DefaultBlock: true,
		},
	}
}

// Run esegue il wizard interattivo su reader/writer e restituisce
// la lista di RuleID da mantenere abilitati (decision=block).
// Le regole non selezionate vengono rimosse dalla policy effettiva.
func Run(r io.Reader, w io.Writer) (blocked []string, err error) {
	fmt.Fprintln(w, "\nConfigurazione policy Guardian")
	fmt.Fprintln(w, "Per ogni azione pericolosa, scegli se bloccarla (default: sì).")
	fmt.Fprintln(w)

	scanner := bufio.NewScanner(r)
	for _, q := range DefaultQuestions() {
		fmt.Fprint(w, q.Prompt())
		scanner.Scan()
		answer := scanner.Text()
		if ParseAnswer(answer, q.DefaultBlock) {
			blocked = append(blocked, q.RuleID)
		}
	}

	fmt.Fprintln(w)
	return blocked, nil
}
