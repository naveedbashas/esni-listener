package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"

	"github.com/example/scte224service/internal/decision"
	"github.com/example/scte224service/internal/domain"
	"github.com/example/scte224service/internal/ingest"
	"github.com/example/scte224service/internal/manifest"
	"github.com/example/scte224service/internal/signals"
	"github.com/example/scte224service/internal/storage"
	"github.com/example/scte224service/internal/temporal/workflows"
)

// Service orchestrates ingest, Temporal scheduling, SDS decisions, and manifest updates.
type Service struct {
	store       *storage.ScheduleStore
	ingest      *ingest.Service
	signalMon   *signals.Monitor
	engine      *decision.Engine
	manipulator *manifest.Manipulator
	audiences   []string

	temporalClient client.Client
	taskQueue      string
}

// New constructs the orchestration service.
func New(temporalClient client.Client, taskQueue string, audiences []string, output io.Writer) *Service {
	store := storage.NewScheduleStore()
	ingestSvc := ingest.New(store)
	engine := decision.NewEngine()
	manipulator := manifest.NewManipulator(output)
	service := &Service{
		store:          store,
		ingest:         ingestSvc,
		engine:         engine,
		manipulator:    manipulator,
		audiences:      append([]string{}, audiences...),
		temporalClient: temporalClient,
		taskQueue:      taskQueue,
	}
	if len(service.audiences) == 0 {
		service.audiences = []string{"urn:scte:224:audience:us"}
	}
	service.signalMon = signals.NewMonitor(store, service.handleSignalMatch)
	return service
}

// IngestMedia parses the SCTE-224 document, updates the schedule store, and ensures workflows run.
func (s *Service) IngestMedia(ctx context.Context, reader io.Reader) (*domain.Media, []string, error) {
	media, err := s.ingest.IngestXML(ctx, reader)
	if err != nil {
		return nil, nil, err
	}
	var workflowIDs []string
	for _, mp := range media.MediaPoints {
		workflowID := buildWorkflowID(mp.ID)
		input := workflows.MediaPointWorkflowInput{
			MediaPointID:     mp.ID,
			MatchTime:        mp.MatchTime,
			Reusable:         mp.Reusable,
			HasApply:         len(mp.ApplyActions) > 0,
			HasRemove:        len(mp.RemoveActions) > 0,
			ExpectedDuration: mp.ExpectedDuration,
		}
		for _, action := range mp.ApplyActions {
			if action == nil || action.Duration <= 0 {
				continue
			}
			input.ApplyDurations = append(input.ApplyDurations, action.Duration)
		}
		options := client.StartWorkflowOptions{
			ID:                       workflowID,
			TaskQueue:                s.taskQueue,
			WorkflowIDReusePolicy:    enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE,
			WorkflowExecutionTimeout: 24 * time.Hour,
		}
		_, err := s.temporalClient.ExecuteWorkflow(ctx, options, workflows.MediaPointWorkflow, input)
		if err != nil {
			var already *serviceerror.WorkflowExecutionAlreadyStarted
			if errors.As(err, &already) {
				workflowIDs = append(workflowIDs, workflowID)
				continue
			}
			return media, workflowIDs, fmt.Errorf("start workflow for %s: %w", mp.ID, err)
		}
		workflowIDs = append(workflowIDs, workflowID)
	}
	return media, workflowIDs, nil
}

// ProcessSignal routes an SCTE-35 event through the monitor.
func (s *Service) ProcessSignal(ctx context.Context, event *domain.SCTE35Event) {
	s.signalMon.Process(ctx, event)
}

// ProcessMediaPoint applies or removes policies for the given media point via the decision engine.

func (s *Service) ProcessMediaPoint(ctx context.Context, input workflows.ProcessActivityInput) error {
	if input.TriggerType == domain.TriggerExpiration {
		decisions := s.engine.RemoveByMediaPoint(input.MediaPointID, s.audiences)
		for _, decision := range decisions {
			s.manipulator.ApplyDecision(decision)
		}
		return nil
	}
	mediaPoint, ok := s.store.MediaPointByID(input.MediaPointID)
	if !ok {
		return fmt.Errorf("media point %s not found", input.MediaPointID)
	}
	trigger := &domain.Trigger{
		MediaPoint: mediaPoint,
		Signal:     input.Event,
		Type:       input.TriggerType,
		OccurredAt: time.Now().UTC(),
	}
	if input.Event != nil && !input.Event.ArrivedAt.IsZero() {
		trigger.OccurredAt = input.Event.ArrivedAt
	}
	decisions := s.engine.Decide(trigger, s.audiences)
	for _, decision := range decisions {
		s.manipulator.ApplyDecision(decision)
	}
	return nil
}

// Snapshot returns the current manifest state.
func (s *Service) Snapshot() map[string]*domain.Decision {
	return s.manipulator.Snapshot()
}

// DescribeAudience provides a friendly manifest summary.
func (s *Service) DescribeAudience(audience string) string {
	return s.manipulator.DescribeAudience(audience)
}

// handleSignalMatch delivers a trigger signal to the Temporal workflow.
func (s *Service) handleSignalMatch(mp *domain.MediaPoint, event *domain.SCTE35Event) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	payload := workflows.TriggerSignalPayload{
		TriggerType: domain.TriggerSignal,
		Event:       event,
	}
	workflowID := buildWorkflowID(mp.ID)
	if err := s.temporalClient.SignalWorkflow(ctx, workflowID, "", workflows.TriggerSignalName, payload); err != nil {
		log.Printf("signal workflow %s: %v", workflowID, err)
	}
}

// TriggerWorkflow manually sends a trigger to the workflow (used by time-based initiation).
func (s *Service) TriggerWorkflow(ctx context.Context, mediaPointID string, triggerType domain.TriggerType) error {
	payload := workflows.TriggerSignalPayload{TriggerType: triggerType}
	return s.temporalClient.SignalWorkflow(ctx, buildWorkflowID(mediaPointID), "", workflows.TriggerSignalName, payload)
}

func buildWorkflowID(mediaPointID string) string {
	clean := strings.NewReplacer(" ", "-", ":", "-", "/", "-", "?", "-", "&", "-").Replace(mediaPointID)
	return fmt.Sprintf("mediapoint-%s", clean)
}
