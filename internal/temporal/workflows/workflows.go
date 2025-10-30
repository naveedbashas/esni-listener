package workflows

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/example/scte224service/internal/domain"
)

const (
	// TriggerSignalName is the Temporal signal channel used to trigger media points.
	TriggerSignalName = "mediapoint-trigger"
	// ProcessMediaPointActivityName is the activity name for applying/removing policies.
	ProcessMediaPointActivityName = "mediapoint-process"
)

// MediaPointWorkflowInput configures the workflow for a specific media point lifecycle.
type MediaPointWorkflowInput struct {
	MediaPointID string
	MatchTime    time.Time
	Reusable     bool
	HasApply     bool
	HasRemove    bool
}

// TriggerSignalPayload is sent via Temporal signals to activate a media point.
type TriggerSignalPayload struct {
	TriggerType domain.TriggerType
	Event       *domain.SCTE35Event
}

// ProcessActivityInput is the payload for the processing activity.
type ProcessActivityInput struct {
	MediaPointID string
	TriggerType  domain.TriggerType
	Event        *domain.SCTE35Event
}

// MediaPointWorkflow orchestrates time and signal triggers for a media point.
func MediaPointWorkflow(ctx workflow.Context, wfInput MediaPointWorkflowInput) error {
	logger := workflow.GetLogger(ctx)
	if !wfInput.HasApply && !wfInput.HasRemove {
		logger.Info("Media point has no apply/remove actions; exiting", "mediaPointID", wfInput.MediaPointID)
		return nil
	}

	activityOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    2 * time.Second,
			BackoffCoefficient: 2,
			MaximumInterval:    10 * time.Second,
			MaximumAttempts:    5,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOpts)

	triggerCh := workflow.GetSignalChannel(ctx, TriggerSignalName)

	var (
		timeFuture    workflow.Future
		timeHandled   bool
		timeScheduled bool
	)

	if !wfInput.MatchTime.IsZero() {
		delay := wfInput.MatchTime.Sub(workflow.Now(ctx))
		if delay < 0 {
			delay = 0
		}
		timeFuture = workflow.NewTimer(ctx, delay)
		timeScheduled = true
	}

	execActivity := func(triggerType domain.TriggerType, event *domain.SCTE35Event) error {
		if triggerType == "" {
			triggerType = domain.TriggerSignal
		}
		activityInput := ProcessActivityInput{
			MediaPointID: wfInput.MediaPointID,
			TriggerType:  triggerType,
			Event:        event,
		}
		return workflow.ExecuteActivity(ctx, ProcessMediaPointActivityName, activityInput).Get(ctx, nil)
	}

	for {
		selector := workflow.NewSelector(ctx)
		if timeScheduled && !timeHandled {
			selector.AddFuture(timeFuture, func(f workflow.Future) {
				if err := execActivity(domain.TriggerTime, nil); err != nil {
					logger.Error("failed to process time trigger", "mediaPointID", wfInput.MediaPointID, "error", err)
				}
				timeHandled = true
			})
		}

		selector.AddReceive(triggerCh, func(c workflow.ReceiveChannel, more bool) {
			var payload TriggerSignalPayload
			c.Receive(ctx, &payload)
			if err := execActivity(payload.TriggerType, payload.Event); err != nil {
				logger.Error("failed to process signal trigger", "mediaPointID", wfInput.MediaPointID, "error", err)
				return
			}
			if !wfInput.Reusable {
				timeHandled = true
			}
		})

		selector.Select(ctx)
		if !wfInput.Reusable && timeHandled {
			return nil
		}
		if !timeScheduled && !wfInput.Reusable {
			// Non-reusable with no schedule and at least one signal processed exits above.
			// To avoid spinning when no signals arrive, block on next signal.
			// However, reaching here means we processed a signal already.
			return nil
		}
	}
}
