package prompt

import (
	"sync"
)

// Response rappresenta la scelta dell'utente al prompt interattivo.
type Response int

const (
	ResponseBlock        Response = iota // blocca questa volta
	ResponseAllowOnce                    // consenti solo questa volta
	ResponseAllowSession                 // consenti per tutta la sessione
	ResponseAllowAlways                  // scrivi regola allow permanente
)

func (r Response) String() string {
	switch r {
	case ResponseBlock:
		return "block"
	case ResponseAllowOnce:
		return "allow_once"
	case ResponseAllowSession:
		return "allow_session"
	case ResponseAllowAlways:
		return "allow_always"
	default:
		return "block"
	}
}

// SessionAllowlist mantiene in memoria i comandi consentiti per la sessione corrente.
// Thread-safe.
type SessionAllowlist struct {
	mu      sync.RWMutex
	entries map[string]map[string]struct{} // agentName → set di comandi
}

func NewSessionAllowlist() *SessionAllowlist {
	return &SessionAllowlist{
		entries: make(map[string]map[string]struct{}),
	}
}

// Add aggiunge un comando all'allowlist di sessione per l'agente dato.
func (s *SessionAllowlist) Add(agentName, command string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.entries[agentName] == nil {
		s.entries[agentName] = make(map[string]struct{})
	}
	s.entries[agentName][command] = struct{}{}
}

// IsAllowed verifica se il comando è nella allowlist di sessione per l'agente.
func (s *SessionAllowlist) IsAllowed(agentName, command string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cmds, ok := s.entries[agentName]
	if !ok {
		return false
	}
	_, allowed := cmds[command]
	return allowed
}
