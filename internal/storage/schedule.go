package storage

import (
	"context"
	"sync"
	"time"

	"github.com/example/scte224service/internal/domain"
)

// ScheduleStore keeps Media and MediaPoints indexed for quick lookup.
type ScheduleStore struct {
	mu          sync.RWMutex
	media       map[string]*domain.Media
	mediaPoints map[string]*domain.MediaPoint
}

// NewScheduleStore constructs an in-memory schedule store.
func NewScheduleStore() *ScheduleStore {
	return &ScheduleStore{
		media:       make(map[string]*domain.Media),
		mediaPoints: make(map[string]*domain.MediaPoint),
	}
}

// UpsertMedia replaces or inserts a media schedule.
func (s *ScheduleStore) UpsertMedia(ctx context.Context, media *domain.Media) {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	s.media[media.ID] = media
	for _, mp := range media.MediaPoints {
		s.mediaPoints[mp.ID] = mp
	}
}

// MediaPoints returns all stored media points.
func (s *ScheduleStore) MediaPoints() []*domain.MediaPoint {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*domain.MediaPoint, 0, len(s.mediaPoints))
	for _, mp := range s.mediaPoints {
		result = append(result, mp)
	}
	return result
}

// MediaPointByID fetches a media point.
func (s *ScheduleStore) MediaPointByID(id string) (*domain.MediaPoint, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	mp, ok := s.mediaPoints[id]
	return mp, ok
}

// NextTimeTriggers returns the subset of media points with match times after now.
func (s *ScheduleStore) NextTimeTriggers(now time.Time) []*domain.MediaPoint {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*domain.MediaPoint
	for _, mp := range s.mediaPoints {
		if mp.MatchTime.IsZero() {
			continue
		}
		if mp.MatchTime.After(now) || mp.MatchTime.Equal(now) {
			result = append(result, mp)
		}
	}
	return result
}
