package scheduler

import (
	"log"
	"math"
	"sync"
	"time"

	"github.com/example/scte224service/internal/domain"
)

// Clock abstracts time retrieval for testability.
type Clock interface {
	Now() time.Time
	AfterFunc(d time.Duration, fn func()) Timer
}

// Timer allows cancellation of scheduled callbacks.
type Timer interface {
	Stop() bool
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now().UTC() }

func (realClock) AfterFunc(d time.Duration, fn func()) Timer { return time.AfterFunc(d, fn) }

// Scheduler schedules time-based triggers for media points.
type Scheduler struct {
	clock     Clock
	timeScale float64
	triggerCh chan<- *domain.Trigger
	mu        sync.Mutex
	timers    map[string]Timer
}

// Option configures a scheduler instance.
type Option func(*Scheduler)

// WithClock injects a custom clock.
func WithClock(clock Clock) Option {
	return func(s *Scheduler) { s.clock = clock }
}

// WithTimeScale compresses or expands delays (e.g., 60 turns 1 hour into 1 minute).
func WithTimeScale(scale float64) Option {
	return func(s *Scheduler) {
		if scale <= 0 {
			scale = 1
		}
		s.timeScale = scale
	}
}

// New constructs a Scheduler.
func New(triggerCh chan<- *domain.Trigger, opts ...Option) *Scheduler {
	s := &Scheduler{
		clock:     realClock{},
		timeScale: 1,
		triggerCh: triggerCh,
		timers:    make(map[string]Timer),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Schedule registers time-based triggers for the provided media points.
func (s *Scheduler) Schedule(mediaPoints []*domain.MediaPoint) {
	for _, mp := range mediaPoints {
		if mp == nil || mp.MatchTime.IsZero() {
			continue
		}
		s.scheduleMediaPoint(mp)
	}
}

func (s *Scheduler) scheduleMediaPoint(mp *domain.MediaPoint) {
	now := s.clock.Now()
	delay := mp.MatchTime.Sub(now)
	if delay <= 0 {
		delay = time.Millisecond
	}
	if s.timeScale != 1 {
		delay = time.Duration(math.Round(float64(delay) / s.timeScale))
		if delay < time.Millisecond {
			delay = time.Millisecond
		}
	}
	cb := func() {
		trigger := &domain.Trigger{
			MediaPoint: mp,
			Type:       domain.TriggerTime,
			OccurredAt: s.clock.Now(),
		}
		select {
		case s.triggerCh <- trigger:
		default:
			log.Printf("scheduler: trigger channel full, dropping event for %s", mp.ID)
		}
	}
	s.mu.Lock()
	if existing, ok := s.timers[mp.ID]; ok {
		existing.Stop()
	}
	s.timers[mp.ID] = s.clock.AfterFunc(delay, cb)
	s.mu.Unlock()
}

// Stop cancels all timers.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, timer := range s.timers {
		if timer != nil {
			timer.Stop()
		}
		delete(s.timers, id)
	}
}
