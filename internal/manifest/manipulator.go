package manifest

import (
	"fmt"
	"io"
	"log"
	"sync"

	"github.com/example/scte224service/internal/domain"
)

// Manipulator applies decisions to viewer manifests.
type Manipulator struct {
	logger *log.Logger
	mu     sync.RWMutex
	state  map[string]*domain.Decision // audience -> last decision
}

// NewManipulator creates a new manifest manipulator writing to the provided writer.
func NewManipulator(out io.Writer) *Manipulator {
	if out == nil {
		out = io.Discard
	}
	return &Manipulator{
		logger: log.New(out, "manifest: ", log.LstdFlags|log.LUTC),
		state:  make(map[string]*domain.Decision),
	}
}

// ApplyDecision mutates the in-memory manifest state based on the decision.
func (m *Manipulator) ApplyDecision(decision *domain.Decision) {
	if decision == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	switch decision.Action {
	case domain.DecisionApply:
		m.state[decision.AudienceID] = decision
		m.logger.Printf("audience=%s apply policy=%s source=%s alt=%s", decision.AudienceID, decision.PolicyID, decision.SourceURI, decision.AlternateURI)
	case domain.DecisionRemove:
		delete(m.state, decision.AudienceID)
		m.logger.Printf("audience=%s remove policy=%s fallback=%s", decision.AudienceID, decision.PolicyID, decision.FallbackURI)
	default:
		m.logger.Printf("audience=%s unknown action for policy=%s", decision.AudienceID, decision.PolicyID)
	}
}

// Snapshot returns a copy of the current manifest state.
func (m *Manipulator) Snapshot() map[string]*domain.Decision {
	m.mu.RLock()
	defer m.mu.RUnlock()
	copy := make(map[string]*domain.Decision, len(m.state))
	for k, v := range m.state {
		copy[k] = v
	}
	return copy
}

// DescribeAudience returns a human-readable description of the audience's manifest.
func (m *Manipulator) DescribeAudience(audience string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if decision, ok := m.state[audience]; ok {
		return fmt.Sprintf("audience=%s alt=%s fallback=%s", audience, decision.AlternateURI, decision.FallbackURI)
	}
	return fmt.Sprintf("audience=%s default", audience)
}
