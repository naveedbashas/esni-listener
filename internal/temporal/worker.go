package temporal

import (
	"context"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/example/scte224service/internal/service"
	"github.com/example/scte224service/internal/temporal/workflows"
)

// Worker encapsulates the Temporal worker lifecycle.
type Worker struct {
	worker worker.Worker
}

// NewWorker registers workflows and activities for media point processing.
func NewWorker(c client.Client, svc *service.Service, taskQueue string) *Worker {
	activities := &activities{svc: svc}
	w := worker.New(c, taskQueue, worker.Options{})
	w.RegisterWorkflow(workflows.MediaPointWorkflow)
	w.RegisterActivityWithOptions(activities.ProcessMediaPoint, activity.RegisterOptions{Name: workflows.ProcessMediaPointActivityName})
	return &Worker{worker: w}
}

// Run executes the worker loop.
func (w *Worker) Run(stopCh <-chan struct{}) error {
	interruptCh := make(chan interface{})
	go func() {
		<-stopCh
		close(interruptCh)
	}()
	return w.worker.Run(interruptCh)
}

// Stop gracefully stops the worker.
func (w *Worker) Stop() {
	w.worker.Stop()
}

type activities struct {
	svc *service.Service
}

func (a *activities) ProcessMediaPoint(ctx context.Context, input workflows.ProcessActivityInput) error {
	return a.svc.ProcessMediaPoint(ctx, input)
}
