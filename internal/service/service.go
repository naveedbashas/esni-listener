package service

import (
	"context"
	"io"
	"sync"

	"github.com/example/scte224service/internal/decision"
	"github.com/example/scte224service/internal/domain"
	"github.com/example/scte224service/internal/ingest"
	"github.com/example/scte224service/internal/manifest"
	"github.com/example/scte224service/internal/scheduler"
	"github.com/example/scte224service/internal/signals"
	"github.com/example/scte224service/internal/storage"
)

// Config configures the orchestration service.
type Config struct {
	Audiences []string
	TimeScale float64
	Output    io.Writer
}

// Service coordinates ingestion, scheduling, signaling, decisioning, and manifest updates.
type Service struct {
	store       *storage.ScheduleStore
	ingest      *ingest.Service
	scheduler   *scheduler.Scheduler
	signalMon   *signals.Monitor
	engine      *decision.Engine
	manipulator *manifest.Manipulator
	audiences   []string

	triggerCh chan *domain.Trigger
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

// New constructs the orchestration service.
func New(cfg Config) *Service {
	triggerCh := make(chan *domain.Trigger, 64)
	store := storage.NewScheduleStore()
	engine := decision.NewEngine()
	manipulator := manifest.NewManipulator(cfg.Output)
	service := &Service{
		store:       store,
		ingest:      ingest.New(store),
		scheduler:   scheduler.New(triggerCh, scheduler.WithTimeScale(cfg.TimeScale)),
		signalMon:   signals.NewMonitor(store, triggerCh),
		engine:      engine,
		manipulator: manipulator,
		audiences:   cfg.Audiences,
		triggerCh:   triggerCh,
		stopCh:      make(chan struct{}),
	}
	if len(service.audiences) == 0 {
		service.audiences = []string{"urn:scte:224:audience:us"}
	}
	return service
}

// Start begins processing of triggers.
func (s *Service) Start() {
	s.wg.Add(1)
	go s.run()
}

func (s *Service) run() {
	defer s.wg.Done()
	for {
		select {
		case trigger := <-s.triggerCh:
			decisions := s.engine.Decide(trigger, s.audiences)
			for _, decision := range decisions {
				s.manipulator.ApplyDecision(decision)
			}
		case <-s.stopCh:
			return
		}
	}
}

// Stop stops the service gracefully.
func (s *Service) Stop() {
	close(s.stopCh)
	s.scheduler.Stop()
	s.wg.Wait()
}

// IngestMedia ingests SCTE-224 XML from the reader and schedules the media points.
func (s *Service) IngestMedia(ctx context.Context, reader io.Reader) (*domain.Media, error) {
	media, err := s.ingest.IngestXML(ctx, reader)
	if err != nil {
		return nil, err
	}
	s.scheduler.Schedule(media.MediaPoints)
	return media, nil
}

// ProcessSignal forwards an SCTE-35 event to the signal monitor.
func (s *Service) ProcessSignal(event *domain.SCTE35Event) {
	s.signalMon.Process(event)
}

// Snapshot provides the manifest state per audience.
func (s *Service) Snapshot() map[string]*domain.Decision {
	return s.manipulator.Snapshot()
}

// DescribeAudience summarises the viewer experience.
func (s *Service) DescribeAudience(audience string) string {
	return s.manipulator.DescribeAudience(audience)
}
