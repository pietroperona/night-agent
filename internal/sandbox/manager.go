package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
)

const (
	// DefaultImage è l'immagine Docker usata se non specificata nella policy.
	DefaultImage = "alpine:3.20"
	// DefaultNetwork disabilita la rete nel container per sicurezza.
	DefaultNetwork = "none"
)

// Config descrive come eseguire un comando nel container sandbox.
type Config struct {
	Image   string // immagine Docker, es: "alpine:3.20"
	Network string // modalità rete: "none" (default) o "bridge"
	WorkDir string // path host da montare come /workspace nel container
}

// ApplyDefaults imposta i valori mancanti con i default sicuri.
func (c *Config) ApplyDefaults() {
	if c.Image == "" {
		c.Image = DefaultImage
	}
	if c.Network == "" {
		c.Network = DefaultNetwork
	}
}

// Result contiene il risultato di un'esecuzione sandbox.
type Result struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

// Manager gestisce l'esecuzione di comandi in container Docker isolati.
type Manager struct{}

// New crea un nuovo SandboxManager.
func New() *Manager {
	return &Manager{}
}

// IsAvailable verifica se Docker è installato e il daemon è in esecuzione.
// Cerca il binario docker anche nei path comuni di macOS (Docker Desktop)
// nel caso in cui il processo chiamante abbia un PATH minimale (es. LaunchAgent).
func (m *Manager) IsAvailable() bool {
	dockerBin := resolveDockerBinary()
	if dockerBin == "" {
		return false
	}
	cmd := exec.Command(dockerBin, "info")
	return cmd.Run() == nil
}

// resolveDockerBinary restituisce il path del binario docker.
// Prova prima exec.LookPath (usa PATH corrente), poi i path fissi di macOS.
func resolveDockerBinary() string {
	if p, err := exec.LookPath("docker"); err == nil {
		return p
	}
	// path comuni su macOS quando il PATH del processo è minimale
	candidates := []string{
		"/usr/local/bin/docker",
		"/opt/homebrew/bin/docker",
		"/Applications/Docker.app/Contents/Resources/bin/docker",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// Execute esegue un comando shell all'interno di un container Docker.
// Il container viene rimosso automaticamente al termine (--rm).
// L'output viene catturato e restituito nel Result.
func (m *Manager) Execute(ctx context.Context, command string, cfg Config) (*Result, error) {
	cfg.ApplyDefaults()

	dockerBin := resolveDockerBinary()
	if dockerBin == "" {
		return nil, fmt.Errorf("Docker non trovato — installa Docker Desktop")
	}

	args := BuildDockerArgs(command, cfg)
	cmd := exec.CommandContext(ctx, dockerBin, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("errore esecuzione Docker: %w", err)
		}
	}

	return &Result{
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
	}, nil
}

// BuildDockerArgs costruisce la lista di argomenti per il comando docker run.
// È esportata per facilitare i test unitari senza richiedere Docker attivo.
func BuildDockerArgs(command string, cfg Config) []string {
	args := []string{
		"run", "--rm",
		"--network", cfg.Network,
	}

	if cfg.WorkDir != "" {
		args = append(args, "-v", cfg.WorkDir+":/workspace:rw")
		args = append(args, "-w", "/workspace")
	}

	args = append(args, cfg.Image)
	args = append(args, "sh", "-c", command)

	return args
}
