package interception_test

import (
	"testing"

	"github.com/pietroperona/agent-guardian/internal/interception"
	"github.com/pietroperona/agent-guardian/internal/policy"
)

func TestNormalize_ShellCommand(t *testing.T) {
	action, err := interception.Normalize("sudo rm -rf /tmp", "/home/user/project", "")
	if err != nil {
		t.Fatalf("atteso nessun errore, ottenuto: %v", err)
	}
	if action.Type != policy.ActionTypeShell {
		t.Errorf("atteso type=shell, ottenuto %s", action.Type)
	}
	if action.Command != "sudo rm -rf /tmp" {
		t.Errorf("atteso command invariato, ottenuto %s", action.Command)
	}
	if action.WorkDir != "/home/user/project" {
		t.Errorf("atteso workdir=/home/user/project, ottenuto %s", action.WorkDir)
	}
}

func TestNormalize_GitCommand(t *testing.T) {
	action, err := interception.Normalize("git push origin main", "/home/user/project", "")
	if err != nil {
		t.Fatalf("atteso nessun errore, ottenuto: %v", err)
	}
	if action.Type != policy.ActionTypeGit {
		t.Errorf("atteso type=git, ottenuto %s", action.Type)
	}
}

func TestNormalize_GitForcePush(t *testing.T) {
	action, err := interception.Normalize("git push --force origin main", "/home/user/project", "")
	if err != nil {
		t.Fatalf("atteso nessun errore, ottenuto: %v", err)
	}
	if action.Type != policy.ActionTypeGit {
		t.Errorf("atteso type=git, ottenuto %s", action.Type)
	}
	if !action.IsForce {
		t.Error("atteso IsForce=true per git push --force")
	}
}

func TestNormalize_FileOperation_Write(t *testing.T) {
	action, err := interception.Normalize("cat > ~/.ssh/id_rsa", "/home/user", "")
	if err != nil {
		t.Fatalf("atteso nessun errore, ottenuto: %v", err)
	}
	if action.Type != policy.ActionTypeFile {
		t.Errorf("atteso type=file, ottenuto %s", action.Type)
	}
}

func TestNormalize_AgentName(t *testing.T) {
	action, err := interception.Normalize("ls -la", "/home/user", "claude-code")
	if err != nil {
		t.Fatalf("atteso nessun errore, ottenuto: %v", err)
	}
	if action.AgentName != "claude-code" {
		t.Errorf("atteso agent_name=claude-code, ottenuto %s", action.AgentName)
	}
}

func TestNormalize_EmptyCommand(t *testing.T) {
	_, err := interception.Normalize("", "/home/user", "")
	if err == nil {
		t.Fatal("atteso errore per comando vuoto, ottenuto nil")
	}
}

func TestNormalize_CommandTrimmed(t *testing.T) {
	action, err := interception.Normalize("  ls -la  ", "/home/user", "")
	if err != nil {
		t.Fatalf("atteso nessun errore, ottenuto: %v", err)
	}
	if action.Command != "ls -la" {
		t.Errorf("atteso comando trimmed, ottenuto '%s'", action.Command)
	}
}

func TestNormalize_GitPushFShortFlag(t *testing.T) {
	action, err := interception.Normalize("git push -f origin main", "/home/user/project", "")
	if err != nil {
		t.Fatalf("atteso nessun errore, ottenuto: %v", err)
	}
	if !action.IsForce {
		t.Error("atteso IsForce=true per git push -f")
	}
}

func TestActionToPolicy_Conversion(t *testing.T) {
	action, _ := interception.Normalize("sudo apt-get install curl", "/home/user", "claude-code")
	policyAction := action.ToPolicyAction()

	if policyAction.Type != "shell" {
		t.Errorf("atteso type=shell, ottenuto %s", policyAction.Type)
	}
	if policyAction.Command != "sudo apt-get install curl" {
		t.Errorf("atteso command invariato, ottenuto %s", policyAction.Command)
	}
}
