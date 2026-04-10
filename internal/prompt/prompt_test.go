package prompt_test

import (
	"testing"

	"github.com/pietroperona/agent-guardian/internal/prompt"
)

// --- PromptResponse ---

func TestPromptResponse_String(t *testing.T) {
	cases := []struct {
		r    prompt.Response
		want string
	}{
		{prompt.ResponseBlock, "block"},
		{prompt.ResponseAllowOnce, "allow_once"},
		{prompt.ResponseAllowSession, "allow_session"},
		{prompt.ResponseAllowAlways, "allow_always"},
	}
	for _, c := range cases {
		if got := c.r.String(); got != c.want {
			t.Errorf("Response(%d).String() = %q, atteso %q", c.r, got, c.want)
		}
	}
}

// --- SessionAllowlist ---

func TestSessionAllowlist_Empty(t *testing.T) {
	sa := prompt.NewSessionAllowlist()
	if sa.IsAllowed("claude", "sudo ls") {
		t.Error("atteso false per allowlist vuota")
	}
}

func TestSessionAllowlist_AddAndCheck(t *testing.T) {
	sa := prompt.NewSessionAllowlist()
	sa.Add("claude", "sudo ls")
	if !sa.IsAllowed("claude", "sudo ls") {
		t.Error("atteso true dopo Add")
	}
}

func TestSessionAllowlist_DifferentAgent(t *testing.T) {
	sa := prompt.NewSessionAllowlist()
	sa.Add("claude", "sudo ls")
	if sa.IsAllowed("codex", "sudo ls") {
		t.Error("allowlist per 'claude' non deve applicarsi a 'codex'")
	}
}

func TestSessionAllowlist_DifferentCommand(t *testing.T) {
	sa := prompt.NewSessionAllowlist()
	sa.Add("claude", "sudo ls")
	if sa.IsAllowed("claude", "sudo rm -rf /") {
		t.Error("allowlist per 'sudo ls' non deve applicarsi a 'sudo rm -rf /'")
	}
}

func TestSessionAllowlist_MultipleCommands(t *testing.T) {
	sa := prompt.NewSessionAllowlist()
	sa.Add("claude", "sudo ls")
	sa.Add("claude", "git push origin main")

	if !sa.IsAllowed("claude", "sudo ls") {
		t.Error("atteso true per 'sudo ls'")
	}
	if !sa.IsAllowed("claude", "git push origin main") {
		t.Error("atteso true per 'git push origin main'")
	}
}

func TestSessionAllowlist_Idempotent(t *testing.T) {
	sa := prompt.NewSessionAllowlist()
	sa.Add("claude", "sudo ls")
	sa.Add("claude", "sudo ls")
	if !sa.IsAllowed("claude", "sudo ls") {
		t.Error("atteso true dopo Add duplicato")
	}
}
