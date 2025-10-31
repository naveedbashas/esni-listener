package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/example/scte224service/internal/domain"
	"github.com/example/scte224service/internal/service"
)

// Server exposes HTTP handlers for the SCTE-224 runtime.
type Server struct {
	svc *service.Service
}

// New constructs a router with bound endpoints.
func New(svc *service.Service) http.Handler {
	server := &Server{svc: svc}
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Heartbeat("/healthz"))

	r.Route("/v1", func(r chi.Router) {
		r.Post("/media", server.handleIngest)
		r.Post("/signals", server.handleSignal)
		r.Get("/policies/active", server.handleActivePolicies)
		r.Get("/health", server.handleHealth)
	})

	return r
}

func (s *Server) handleIngest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		respondError(w, http.StatusBadRequest, "failed to read body: %v", err)
		return
	}
	defer r.Body.Close()
	media, workflowIDs, err := s.svc.IngestMedia(ctx, bytesReader(body))
	if err != nil {
		respondError(w, http.StatusBadRequest, "ingest failed: %v", err)
		return
	}
	resp := map[string]any{
		"mediaId":         media.ID,
		"mediaPointCount": len(media.MediaPoints),
		"workflowIds":     workflowIDs,
	}
	respondJSON(w, http.StatusCreated, resp)
}

func (s *Server) handleSignal(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req signalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid signal payload: %v", err)
		return
	}
	if req.ArrivedAt == nil {
		now := time.Now().UTC()
		req.ArrivedAt = &now
	}
	event := &domain.SCTE35Event{
		EventID:             req.EventID,
		SegmentationType:    req.SegmentationTypeID,
		SegmentationEventID: req.SegmentationEventID,
		PrivateIndicator:    req.PrivateIndicator,
		ArrivedAt:           *req.ArrivedAt,
	}
	s.svc.ProcessSignal(ctx, event)
	respondJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

func (s *Server) handleActivePolicies(w http.ResponseWriter, r *http.Request) {
	snapshot := s.svc.Snapshot()
	respondJSON(w, http.StatusOK, snapshot)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type signalRequest struct {
	EventID             string     `json:"eventId"`
	SegmentationTypeID  string     `json:"segmentationTypeId"`
	SegmentationEventID string     `json:"segmentationEventId"`
	PrivateIndicator    string     `json:"privateIndicator"`
	ArrivedAt           *time.Time `json:"arrivedAt"`
}

func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func respondError(w http.ResponseWriter, status int, format string, args ...any) {
	respondJSON(w, status, map[string]string{
		"error": fmt.Sprintf(format, args...),
	})
}

func bytesReader(b []byte) io.Reader {
	return bytes.NewReader(b)
}
