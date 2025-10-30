package decision

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/example/scte224service/internal/domain"
)

// Engine maintains policy state and produces decisions for triggers.
type Engine struct {
	mu     sync.RWMutex
	active map[string]map[string]*domain.Decision // audience -> policyID -> decision
}

// NewEngine constructs a decision engine.
func NewEngine() *Engine {
	return &Engine{
		active: make(map[string]map[string]*domain.Decision),
	}
}

// Decide evaluates the trigger for the provided audiences.
func (e *Engine) Decide(trigger *domain.Trigger, audiences []string) []*domain.Decision {
	if trigger == nil || trigger.MediaPoint == nil {
		return nil
	}
	var decisions []*domain.Decision
	for _, audience := range audiences {
		if decision := e.evaluateApply(trigger, audience); decision != nil {
			decisions = append(decisions, decision)
			e.storeDecision(decision)
		}
		removals := e.evaluateRemovals(trigger, audience)
		for _, removal := range removals {
			decisions = append(decisions, removal)
			e.storeDecision(removal)
		}
	}
	return decisions
}

// RemoveByMediaPoint removes all active policies that originated from the given media point.
func (e *Engine) RemoveByMediaPoint(mediaPointID string, audiences []string) []*domain.Decision {
	if mediaPointID == "" {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	var removals []*domain.Decision
	now := time.Now().UTC()
	for _, audience := range audiences {
		activePolicies := e.active[audience]
		if len(activePolicies) == 0 {
			continue
		}
		for policyID, decision := range activePolicies {
			if decision == nil || decision.MediaPointID != mediaPointID {
				continue
			}
			removal := &domain.Decision{
				MediaPointID:    decision.MediaPointID,
				PolicyID:        decision.PolicyID,
				ViewingPolicyID: decision.ViewingPolicyID,
				AudienceID:      decision.AudienceID,
				Priority:        decision.Priority,
				SourceURI:       decision.SourceURI,
				AlternateURI:    decision.AlternateURI,
				FallbackURI:     decision.FallbackURI,
				TriggeredAt:     now,
				TriggerType:     domain.TriggerExpiration,
				Action:          domain.DecisionRemove,
			}
			removals = append(removals, removal)
			delete(activePolicies, policyID)
		}
		if len(activePolicies) == 0 {
			delete(e.active, audience)
		}
	}
	return removals
}

func (e *Engine) evaluateApply(trigger *domain.Trigger, audience string) *domain.Decision {
	mp := trigger.MediaPoint
	if len(mp.ApplyActions) == 0 {
		return nil
	}
	apply := selectHighestPriority(mp.ApplyActions)
	if apply == nil || apply.Policy == nil {
		return nil
	}
	viewingPolicy := selectViewingPolicy(apply.Policy, audience)
	if viewingPolicy == nil {
		return nil
	}
	content := viewingPolicy.Content
	if content == nil {
		return nil
	}
	return &domain.Decision{
		MediaPointID:    mp.ID,
		PolicyID:        apply.Policy.ID,
		ViewingPolicyID: viewingPolicy.ID,
		AudienceID:      audience,
		Priority:        apply.Priority,
		SourceURI:       content.SourceURI,
		AlternateURI:    content.AlternateURI,
		FallbackURI:     content.FallbackURI,
		TriggeredAt:     triggerTimestamp(trigger),
		TriggerType:     trigger.Type,
		Action:          domain.DecisionApply,
	}
}

func (e *Engine) evaluateRemovals(trigger *domain.Trigger, audience string) []*domain.Decision {
	mp := trigger.MediaPoint
	if len(mp.RemoveActions) == 0 {
		return nil
	}
	var removals []*domain.Decision
	e.mu.RLock()
	activeForAudience := e.active[audience]
	e.mu.RUnlock()
	if len(activeForAudience) == 0 {
		return nil
	}
	for _, action := range mp.RemoveActions {
		if action == nil || action.PolicyRef == "" {
			continue
		}
		if decision := lookupDecision(activeForAudience, action.PolicyRef); decision != nil {
			removal := &domain.Decision{
				MediaPointID:    mp.ID,
				PolicyID:        decision.PolicyID,
				ViewingPolicyID: decision.ViewingPolicyID,
				AudienceID:      audience,
				Priority:        decision.Priority,
				SourceURI:       decision.SourceURI,
				AlternateURI:    decision.AlternateURI,
				FallbackURI:     decision.FallbackURI,
				TriggeredAt:     triggerTimestamp(trigger),
				TriggerType:     trigger.Type,
				Action:          domain.DecisionRemove,
			}
			removals = append(removals, removal)
		}
	}
	return removals
}

func selectHighestPriority(actions []*domain.ApplyAction) *domain.ApplyAction {
	if len(actions) == 0 {
		return nil
	}
	filtered := make([]*domain.ApplyAction, 0, len(actions))
	for _, action := range actions {
		if action != nil {
			filtered = append(filtered, action)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Priority > filtered[j].Priority
	})
	return filtered[0]
}

func selectViewingPolicy(policy *domain.Policy, audience string) *domain.ViewingPolicy {
	if policy == nil {
		return nil
	}
	var fallback *domain.ViewingPolicy
	for _, vp := range policy.ViewingPolicies {
		if vp == nil || vp.Content == nil {
			continue
		}
		if audience != "" && strings.EqualFold(vp.AudienceID, audience) {
			return vp
		}
		if vp.AudienceID == "" && fallback == nil {
			fallback = vp
		}
	}
	return fallback
}

func (e *Engine) storeDecision(decision *domain.Decision) {
	if decision == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.active[decision.AudienceID]; !ok {
		e.active[decision.AudienceID] = make(map[string]*domain.Decision)
	}
	if decision.Action == domain.DecisionApply {
		e.active[decision.AudienceID][decision.PolicyID] = decision
	} else if decision.Action == domain.DecisionRemove {
		delete(e.active[decision.AudienceID], decision.PolicyID)
	}
}

func triggerTimestamp(trigger *domain.Trigger) time.Time {
	if trigger == nil {
		return time.Now().UTC()
	}
	if !trigger.OccurredAt.IsZero() {
		return trigger.OccurredAt
	}
	return time.Now().UTC()
}

func lookupDecision(active map[string]*domain.Decision, policyRef string) *domain.Decision {
	if active == nil {
		return nil
	}
	if decision, ok := active[policyRef]; ok {
		return decision
	}
	if strings.HasPrefix(policyRef, "urn:") {
		parts := strings.Split(policyRef, ":")
		trimmed := parts[len(parts)-1]
		if decision, ok := active[trimmed]; ok {
			return decision
		}
	}
	return nil
}
