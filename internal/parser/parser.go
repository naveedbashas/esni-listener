package parser

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/example/scte224service/internal/domain"
)

// MediaParser reads SCTE-224 Media documents and builds domain models.
type MediaParser struct{}

// ParseFile reads an XML document from disk.
func (p *MediaParser) ParseFile(path string) (*domain.Media, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open media file: %w", err)
	}
	defer file.Close()
	return p.Parse(file)
}

// Parse consumes an io.Reader containing SCTE-224 XML.
func (p *MediaParser) Parse(r io.Reader) (*domain.Media, error) {
	var doc mediaDocument
	decoder := xml.NewDecoder(r)
	decoder.CharsetReader = charsetReader
	if err := decoder.Decode(&doc); err != nil {
		return nil, fmt.Errorf("decode media document: %w", err)
	}
	return translateMedia(doc)
}

// charsetReader handles UTF-8 only but keeps extension point.
func charsetReader(_ string, input io.Reader) (io.Reader, error) {
	return input, nil
}

type mediaDocument struct {
	XMLName     xml.Name     `xml:"{http://www.scte.org/schemas/224}Media"`
	ID          string       `xml:"id,attr"`
	Description string       `xml:"description,attr"`
	LastUpdated string       `xml:"lastUpdated,attr"`
	Effective   string       `xml:"effective,attr"`
	Expires     string       `xml:"expires,attr"`
	MediaPoints []mediaPoint `xml:"MediaPoint"`
}

type mediaPoint struct {
	ID               string          `xml:"id,attr"`
	Description      string          `xml:"description,attr"`
	Effective        string          `xml:"effective,attr"`
	Expires          string          `xml:"expires,attr"`
	MatchTime        string          `xml:"matchTime,attr"`
	ExpectedDuration string          `xml:"expectedDuration,attr"`
	Reusable         string          `xml:"reusable,attr"`
	AltIDs           []altID         `xml:"AltID"`
	MatchSignals     []matchSignal   `xml:"MatchSignal"`
	Apply            []applyElement  `xml:"Apply"`
	Remove           []removeElement `xml:"Remove"`
}

type altID struct {
	Type  string `xml:"type,attr"`
	Value string `xml:",chardata"`
}

type matchSignal struct {
	Match   string         `xml:"match,attr"`
	Schema  string         `xml:"schema,attr"`
	Asserts []signalAssert `xml:"Assert"`
}

type signalAssert struct {
	Expression string `xml:",chardata"`
}

type applyElement struct {
	Priority string        `xml:"priority,attr"`
	Duration string        `xml:"duration,attr"`
	Policy   policyElement `xml:"Policy"`
}

type removeElement struct {
	Policies []removePolicy `xml:"Policy"`
}

type removePolicy struct {
	Href string `xml:"{http://www.w3.org/1999/xlink}href,attr"`
}

type policyElement struct {
	ID              string          `xml:"id,attr"`
	ViewingPolicies []viewingPolicy `xml:"ViewingPolicy"`
}

type viewingPolicy struct {
	ID       string          `xml:"id,attr"`
	Audience audienceElement `xml:"Audience"`
	Content  contentElement  `xml:"{http://qplive.com/schemas/224/action}Content"`
}

type audienceElement struct {
	ID string `xml:"id,attr"`
}

type contentElement struct {
	SourceURI    string `xml:"{http://qplive.com/schemas/224/action}SourceURI"`
	AlternateURI string `xml:"{http://qplive.com/schemas/224/action}AlternateURI"`
	FallbackURI  string `xml:"{http://qplive.com/schemas/224/action}FallbackURI"`
}

func translateMedia(doc mediaDocument) (*domain.Media, error) {
	if doc.ID == "" {
		return nil, errors.New("media id is required")
	}
	media := &domain.Media{
		ID:          doc.ID,
		Description: doc.Description,
	}
	var err error
	if media.LastUpdated, err = parseTime(doc.LastUpdated); err != nil {
		return nil, fmt.Errorf("media lastUpdated: %w", err)
	}
	if media.Effective, err = parseTime(doc.Effective); err != nil {
		return nil, fmt.Errorf("media effective: %w", err)
	}
	if media.Expires, err = parseTime(doc.Expires); err != nil {
		return nil, fmt.Errorf("media expires: %w", err)
	}
	for _, mp := range doc.MediaPoints {
		converted, err := translateMediaPoint(mp)
		if err != nil {
			return nil, fmt.Errorf("media point %s: %w", mp.ID, err)
		}
		media.MediaPoints = append(media.MediaPoints, converted)
	}
	return media, nil
}

func translateMediaPoint(mp mediaPoint) (*domain.MediaPoint, error) {
	if mp.ID == "" {
		return nil, errors.New("id attribute is required")
	}
	result := &domain.MediaPoint{
		ID:          mp.ID,
		Description: mp.Description,
		AltIDs:      make(map[string]string),
		Reusable:    strings.EqualFold(mp.Reusable, "true"),
	}
	var err error
	if result.Effective, err = parseTime(mp.Effective); err != nil && mp.Effective != "" {
		return nil, fmt.Errorf("effective: %w", err)
	}
	if result.Expires, err = parseTime(mp.Expires); err != nil && mp.Expires != "" {
		return nil, fmt.Errorf("expires: %w", err)
	}
	if result.MatchTime, err = parseTime(mp.MatchTime); err != nil && mp.MatchTime != "" {
		return nil, fmt.Errorf("matchTime: %w", err)
	}
	if result.ExpectedDuration, err = parseDuration(mp.ExpectedDuration); err != nil && mp.ExpectedDuration != "" {
		return nil, fmt.Errorf("expectedDuration: %w", err)
	}
	for _, alt := range mp.AltIDs {
		if alt.Type != "" {
			result.AltIDs[alt.Type] = strings.TrimSpace(alt.Value)
		}
	}
	for _, ms := range mp.MatchSignals {
		result.MatchSignals = append(result.MatchSignals, translateMatchSignal(ms))
	}
	for _, apply := range mp.Apply {
		action, err := translateApply(apply)
		if err != nil {
			return nil, err
		}
		result.ApplyActions = append(result.ApplyActions, action)
	}
	for _, rem := range mp.Remove {
		for _, pol := range rem.Policies {
			result.RemoveActions = append(result.RemoveActions, &domain.RemoveAction{PolicyRef: pol.Href})
		}
	}
	return result, nil
}

func translateMatchSignal(ms matchSignal) *domain.MatchSignal {
	result := &domain.MatchSignal{Match: ms.Match, Schema: ms.Schema}
	for _, assertion := range ms.Asserts {
		result.Asserts = append(result.Asserts, &domain.SignalAssertion{Expression: strings.TrimSpace(assertion.Expression)})
	}
	return result
}

func translateApply(apply applyElement) (*domain.ApplyAction, error) {
	priority, err := parseInt(apply.Priority, 0)
	if err != nil {
		return nil, fmt.Errorf("apply priority: %w", err)
	}
	duration, err := parseDuration(apply.Duration)
	if err != nil && apply.Duration != "" {
		return nil, fmt.Errorf("apply duration: %w", err)
	}
	policy := &domain.Policy{ID: apply.Policy.ID}
	for _, vp := range apply.Policy.ViewingPolicies {
		policy.ViewingPolicies = append(policy.ViewingPolicies, translateViewingPolicy(vp))
	}
	return &domain.ApplyAction{
		Priority: priority,
		Duration: duration,
		Policy:   policy,
	}, nil
}

func translateViewingPolicy(vp viewingPolicy) *domain.ViewingPolicy {
	content := &domain.Content{
		SourceURI:    strings.TrimSpace(vp.Content.SourceURI),
		AlternateURI: strings.TrimSpace(vp.Content.AlternateURI),
		FallbackURI:  strings.TrimSpace(vp.Content.FallbackURI),
	}
	audienceID := strings.TrimSpace(vp.Audience.ID)
	return &domain.ViewingPolicy{
		ID:         vp.ID,
		AudienceID: audienceID,
		Content:    content,
	}
}

func parseTime(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, nil
	}
	if strings.HasSuffix(value, "Z") {
		value = strings.TrimSuffix(value, "Z") + "+00:00"
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, err
	}
	return t.UTC(), nil
}

func parseDuration(value string) (time.Duration, error) {
	if strings.TrimSpace(value) == "" {
		return 0, nil
	}
	if !strings.HasPrefix(value, "P") {
		return 0, fmt.Errorf("duration must start with 'P'")
	}
	value = strings.TrimPrefix(value, "P")
	var (
		total  time.Duration
		num    strings.Builder
		inTime bool
	)
	flush := func(multiplier time.Duration) {
		if num.Len() == 0 {
			return
		}
		v, _ := parseInt(num.String(), 0)
		total += time.Duration(v) * multiplier
		num.Reset()
	}
	for _, r := range value {
		switch r {
		case 'T':
			inTime = true
		case 'D':
			flush(24 * time.Hour)
		case 'H':
			if !inTime {
				return 0, fmt.Errorf("hours specified before time designator")
			}
			flush(time.Hour)
		case 'M':
			if !inTime {
				return 0, fmt.Errorf("month parsing not supported")
			}
			flush(time.Minute)
		case 'S':
			if !inTime {
				return 0, fmt.Errorf("seconds specified before time designator")
			}
			flush(time.Second)
		default:
			num.WriteRune(r)
		}
	}
	return total, nil
}

func parseInt(value string, fallback int) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, nil
	}
	i, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	return i, nil
}
