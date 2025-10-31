package ingest

import (
	"context"
	"fmt"
	"io"

	"github.com/example/scte224service/internal/domain"
	"github.com/example/scte224service/internal/parser"
	"github.com/example/scte224service/internal/storage"
)

// Service performs ingest of SCTE-224 documents via the ESNI API boundary.
type Service struct {
	parser *parser.MediaParser
	store  *storage.ScheduleStore
}

// New creates a new ingest service.
func New(store *storage.ScheduleStore) *Service {
	return &Service{
		parser: &parser.MediaParser{},
		store:  store,
	}
}

// IngestXML parses and stores the SCTE-224 document, returning the materialized media.
func (s *Service) IngestXML(ctx context.Context, reader io.Reader) (*domain.Media, error) {
	media, err := s.parser.Parse(reader)
	if err != nil {
		return nil, fmt.Errorf("parse SCTE-224: %w", err)
	}
	s.store.UpsertMedia(ctx, media)
	return media, nil
}
