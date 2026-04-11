package wizard_test

import (
	"strings"
	"testing"

	"github.com/pietroperona/night-agent/internal/wizard"
)

func TestParseAnswer_Yes(t *testing.T) {
	for _, input := range []string{"y", "Y", "yes", "YES", "s", "S", "si", "SI", ""} {
		if !wizard.ParseAnswer(input, true) {
			t.Errorf("ParseAnswer(%q, default=true) atteso true", input)
		}
	}
}

func TestParseAnswer_No(t *testing.T) {
	for _, input := range []string{"n", "N", "no", "NO"} {
		if wizard.ParseAnswer(input, true) {
			t.Errorf("ParseAnswer(%q, default=true) atteso false", input)
		}
	}
}

func TestParseAnswer_DefaultFalse(t *testing.T) {
	if wizard.ParseAnswer("", false) {
		t.Error("ParseAnswer('', default=false) atteso false")
	}
}

func TestQuestion_Format(t *testing.T) {
	q := wizard.Question{
		Label:        "block_sudo",
		Description:  "sudo è disabilitato per gli agenti AI",
		DefaultBlock: true,
	}
	prompt := q.Prompt()
	// il prompt contiene ANSI codes — verifichiamo che contenga il testo chiave stripped
	stripped := wizard.StripANSI(prompt)
	if !strings.Contains(stripped, "Bloccare") {
		t.Errorf("prompt non contiene 'Bloccare', ottenuto: %q", stripped)
	}
}

func TestQuestion_Prompt_DefaultBlock(t *testing.T) {
	q := wizard.Question{Description: "sudo disabilitato", DefaultBlock: true}
	stripped := wizard.StripANSI(q.Prompt())
	if !strings.Contains(stripped, "S") {
		t.Errorf("atteso 'S' maiuscola per DefaultBlock=true, ottenuto: %q", stripped)
	}
}

func TestQuestion_Prompt_DefaultAllow(t *testing.T) {
	q := wizard.Question{Description: "git push su main", DefaultBlock: false}
	stripped := wizard.StripANSI(q.Prompt())
	if !strings.Contains(stripped, "N") {
		t.Errorf("atteso 'N' maiuscola per DefaultBlock=false, ottenuto: %q", stripped)
	}
}

func TestDefaultQuestions_NotEmpty(t *testing.T) {
	qs := wizard.DefaultQuestions()
	if len(qs) == 0 {
		t.Error("DefaultQuestions non deve essere vuota")
	}
}

func TestDefaultQuestions_HaveLabels(t *testing.T) {
	for i, q := range wizard.DefaultQuestions() {
		if q.Label == "" {
			t.Errorf("domanda %d non ha Label", i)
		}
		if q.Description == "" {
			t.Errorf("domanda %d non ha Description", i)
		}
		if q.RuleID == "" {
			t.Errorf("domanda %d non ha RuleID", i)
		}
	}
}
