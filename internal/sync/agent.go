package sync

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/pietroperona/night-agent/internal/audit"
	"github.com/pietroperona/night-agent/internal/cloudconfig"
)

const batchSize = 100

// IngestRequest è il payload inviato alla Cloud API.
type IngestRequest struct {
	MachineID  string        `json:"machine_id"`
	Batch      []audit.Event `json:"batch"`
	PolicyYAML string        `json:"policy_yaml,omitempty"`
}

// IngestResponse è la risposta della Cloud API.
type IngestResponse struct {
	Received int    `json:"received"`
	Cursor   string `json:"cursor"`
}

// Agent legge eventi da logPath, li batchizza e li invia all'API cloud.
type Agent struct {
	cfgPath          string
	logPath          string
	policyPath       string // path file policy locale/globale da includere nel payload (vuoto = omit)
	endpointOverride string // se non vuoto, sovrascrive cfg.Endpoint (usato nei test)
	client           *http.Client
}

// NewAgent crea un Agent con HTTP client di default (timeout 30s).
func NewAgent(cfgPath, logPath string) *Agent {
	return &Agent{
		cfgPath: cfgPath,
		logPath: logPath,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// WithEndpoint sovrascrive l'endpoint letto dalla config. Usato nei test.
func (a *Agent) WithEndpoint(endpoint string) *Agent {
	a.endpointOverride = endpoint
	return a
}

// WithPolicyPath imposta il path della policy locale/globale da includere nel sync.
// Da chiamare solo quando la policy è di sorgente locale o globale (non cloud).
func (a *Agent) WithPolicyPath(path string) *Agent {
	a.policyPath = path
	return a
}

// readPolicyYAML legge il file policy e restituisce il contenuto come stringa.
// Ritorna stringa vuota in caso di errore — fail-open.
func (a *Agent) readPolicyYAML() string {
	if a.policyPath == "" {
		return ""
	}
	data, err := os.ReadFile(a.policyPath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// SyncOnce legge tutti gli eventi post-cursor, li invia in batch e aggiorna il cursore.
// Ritorna ErrUnauthorized se l'API risponde 401.
// Non modifica il daemon locale in caso di errore (fail-open).
func (a *Agent) SyncOnce() error {
	cfg, err := cloudconfig.Load(a.cfgPath)
	if err != nil {
		return fmt.Errorf("caricamento config: %w", err)
	}

	events, err := audit.ReadAll(a.logPath)
	if err != nil {
		return fmt.Errorf("lettura log: %w", err)
	}

	// filtra eventi post-cursor
	pending := eventsAfterCursor(events, cfg.Cursor)
	if len(pending) == 0 {
		return nil // niente da inviare
	}

	// invia in batch da batchSize — policy inclusa solo nel primo batch
	var lastCursor string
	policyYAML := a.readPolicyYAML()
	for i := 0; i < len(pending); i += batchSize {
		end := i + batchSize
		if end > len(pending) {
			end = len(pending)
		}
		batch := pending[i:end]

		// includi policy solo nel primo batch, poi ometti
		py := ""
		if i == 0 {
			py = policyYAML
		}
		cursor, err := a.sendBatch(cfg, batch, py)
		if err != nil {
			return err
		}
		lastCursor = cursor
	}

	// aggiorna cursore su disco
	if lastCursor != "" {
		if err := cloudconfig.UpdateCursor(a.cfgPath, lastCursor); err != nil {
			return fmt.Errorf("aggiornamento cursore: %w", err)
		}
	}
	return nil
}

// sendBatch invia un singolo batch all'API e ritorna il cursore ricevuto.
func (a *Agent) sendBatch(cfg *cloudconfig.Config, batch []audit.Event, policyYAML string) (string, error) {
	req := IngestRequest{
		MachineID:  cfg.MachineID,
		Batch:      batch,
		PolicyYAML: policyYAML,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("serializzazione batch: %w", err)
	}

	endpoint := cfg.Endpoint
	if a.endpointOverride != "" {
		endpoint = a.endpointOverride
	}
	httpReq, err := http.NewRequest(http.MethodPost, endpoint+"/api/ingest", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creazione richiesta: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+cfg.Token)

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("invio batch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return "", fmt.Errorf("token non valido (401) — esegui 'nightagent cloud connect <TOKEN>' per rinnovare")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("risposta API inattesa: %d", resp.StatusCode)
	}

	var res IngestResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", fmt.Errorf("parsing risposta API: %w", err)
	}
	return res.Cursor, nil
}

// eventsAfterCursor ritorna gli eventi che vengono dopo l'evento con ID=cursor.
// Se cursor è vuoto, ritorna tutti gli eventi.
func eventsAfterCursor(events []audit.Event, cursor string) []audit.Event {
	if cursor == "" {
		return events
	}
	for i, e := range events {
		if e.ID == cursor {
			return events[i+1:]
		}
	}
	// cursor non trovato nel log → invia tutto (caso: log ruotato o primo sync)
	return events
}
