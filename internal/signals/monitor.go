package signals

import (
	"context"
	"strings"
	"time"

	"github.com/example/scte224service/internal/domain"
	"github.com/example/scte224service/internal/storage"
)

// TriggerHandler is invoked for each media point matched by a signal.
type TriggerHandler func(mp *domain.MediaPoint, event *domain.SCTE35Event)

// Monitor evaluates incoming SCTE-35 events against stored media point match rules.
type Monitor struct {
	store   *storage.ScheduleStore
	handler TriggerHandler
}

// NewMonitor creates a signal monitor.
func NewMonitor(store *storage.ScheduleStore, handler TriggerHandler) *Monitor {
	return &Monitor{store: store, handler: handler}
}

// Process ingests an SCTE-35 event and invokes the handler for each match.
func (m *Monitor) Process(ctx context.Context, event *domain.SCTE35Event) {
	if event == nil {
		return
	}
	if event.ArrivedAt.IsZero() {
		event.ArrivedAt = time.Now().UTC()
	}
	if m.handler == nil {
		return
	}
	mediaPoints := m.store.MediaPoints()
	for _, mp := range mediaPoints {
		if len(mp.MatchSignals) == 0 {
			continue
		}
		if matchesMediaPoint(mp, event) {
			select {
			case <-ctx.Done():
				return
			default:
			}
			m.handler(mp, event)
		}
	}
}

func matchesMediaPoint(mp *domain.MediaPoint, event *domain.SCTE35Event) bool {
	for _, matcher := range mp.MatchSignals {
		if evaluateMatcher(matcher, event) {
			return true
		}
	}
	return false
}

func evaluateMatcher(matcher *domain.MatchSignal, event *domain.SCTE35Event) bool {
	if matcher == nil {
		return false
	}
	matchMode := strings.ToUpper(strings.TrimSpace(matcher.Match))
	if matchMode == "" {
		matchMode = "ALL"
	}
	matched := 0
	for _, assertion := range matcher.Asserts {
		if assertion == nil {
			continue
		}
		if evaluateAssertion(assertion.Expression, event) {
			matched++
		} else if matchMode == "ALL" {
			return false
		}
	}
	if matchMode == "ANY" {
		return matched > 0
	}
	return matched == len(matcher.Asserts)
}

func evaluateAssertion(expression string, event *domain.SCTE35Event) bool {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return false
	}
	if strings.Contains(expression, "@spliceEventId") {
		value := extractAttributeValue(expression, "spliceEventId")
		return equalsHexInsensitive(event.EventID, value)
	}
	if strings.Contains(expression, "@segmentationEventId") {
		value := extractAttributeValue(expression, "segmentationEventId")
		return equalsHexInsensitive(event.SegmentationEventID, value)
	}
	if strings.Contains(expression, "@segmentationTypeId") {
		value := extractAttributeValue(expression, "segmentationTypeId")
		return equalsHexInsensitive(event.SegmentationType, value)
	}
	if strings.Contains(expression, "contains(@privateIndicator") {
		needle := extractContainsValue(expression)
		return needle != "" && strings.Contains(strings.ToLower(event.PrivateIndicator), strings.ToLower(needle))
	}
	return false
}

func extractAttributeValue(expression, attribute string) string {
	needle := "@" + attribute + "="
	idx := strings.Index(expression, needle)
	if idx == -1 {
		return ""
	}
	valuePart := expression[idx+len(needle):]
	valuePart = strings.TrimSpace(valuePart)
	if valuePart == "" {
		return ""
	}
	quote := valuePart[0]
	if quote != '\'' && quote != '"' {
		return ""
	}
	valuePart = valuePart[1:]
	end := strings.IndexRune(valuePart, rune(quote))
	if end == -1 {
		return ""
	}
	return strings.TrimSpace(valuePart[:end])
}

func extractContainsValue(expression string) string {
	start := strings.Index(expression, ",")
	if start == -1 {
		return ""
	}
	rest := expression[start+1:]
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return ""
	}
	quote := rest[0]
	if quote != '\'' && quote != '"' {
		return ""
	}
	rest = rest[1:]
	end := strings.IndexRune(rest, rune(quote))
	if end == -1 {
		return ""
	}
	return rest[:end]
}

func equalsHexInsensitive(a, b string) bool {
	return normalizeHex(a) == normalizeHex(b)
}

func normalizeHex(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if strings.HasPrefix(value, "0x") {
		return value
	}
	if value == "" {
		return value
	}
	return "0x" + value
}
