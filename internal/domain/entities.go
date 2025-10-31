package domain

import (
	"time"
)

// Media models the SCTE-224 Media element.
type Media struct {
	ID          string
	Description string
	LastUpdated time.Time
	Effective   time.Time
	Expires     time.Time
	MediaPoints []*MediaPoint
}

// MediaPoint describes the switching instructions and signal triggers.
type MediaPoint struct {
	ID               string
	Description      string
	Effective        time.Time
	Expires          time.Time
	MatchTime        time.Time
	ExpectedDuration time.Duration
	Reusable         bool
	AltIDs           map[string]string
	MatchSignals     []*MatchSignal
	ApplyActions     []*ApplyAction
	RemoveActions    []*RemoveAction
}

// MatchSignal conveys SCTE-35 assertion requirements.
type MatchSignal struct {
	Match   string
	Schema  string
	Asserts []*SignalAssertion
}

// SignalAssertion captures XPath expression semantics in simplified form.
type SignalAssertion struct {
	Expression string
}

// ApplyAction instructs how to manipulate the policy stack.
type ApplyAction struct {
	Priority int
	Duration time.Duration
	Policy   *Policy
}

// RemoveAction removes an existing policy by reference.
type RemoveAction struct {
	PolicyRef string
}

// Policy groups one or more viewing policies.
type Policy struct {
	ID              string
	ViewingPolicies []*ViewingPolicy
}

// ViewingPolicy binds audience targeting with playback content.
type ViewingPolicy struct {
	ID         string
	AudienceID string
	Content    *Content
}

// Content describes possible URIs for playback routing.
type Content struct {
	SourceURI    string
	AlternateURI string
	FallbackURI  string
}

// Decision encapsulates the output of the SDS.
type Decision struct {
	MediaPointID    string
	PolicyID        string
	ViewingPolicyID string
	AudienceID      string
	Priority        int
	SourceURI       string
	AlternateURI    string
	FallbackURI     string
	TriggeredAt     time.Time
	TriggerType     TriggerType
	Action          DecisionAction
}

// DecisionAction enumerates policy actions.
type DecisionAction string

const (
	// DecisionApply represents applying a policy.
	DecisionApply DecisionAction = "apply"
	// DecisionRemove represents removing a policy.
	DecisionRemove DecisionAction = "remove"
)

// Trigger represents either a scheduled or signal-based activation request.
type Trigger struct {
	MediaPoint *MediaPoint
	Signal     *SCTE35Event
	Type       TriggerType
	OccurredAt time.Time
}

// TriggerType categorises the origin of an event.
type TriggerType string

const (
	// TriggerTime indicates a scheduled/time-based activation.
	TriggerTime TriggerType = "time"
	// TriggerSignal indicates an in-band signaling activation.
	TriggerSignal TriggerType = "signal"
	// TriggerExpiration indicates an automatic expiration of a media point or apply duration.
	TriggerExpiration TriggerType = "expiration"
)

// SCTE35Event is a simplified representation of an in-band signal.
type SCTE35Event struct {
	EventID             string
	SegmentationType    string
	SegmentationEventID string
	PrivateIndicator    string
	RawXML              string
	ArrivedAt           time.Time
}
