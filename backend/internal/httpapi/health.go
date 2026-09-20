package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type healthResponse struct {
	Status   string `json:"status"`
	Database string `json:"database"`
	Version  string `json:"version"`
	Time     string `json:"time"`
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	dbStatus := "ok"

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := pingDatabase(ctx, s); err != nil {
		dbStatus = "unavailable"
	}

	status := "ok"
	if dbStatus != "ok" {
		status = "degraded"
	}

	w.Header().Set("Content-Type", "application/json")
	if status != "ok" {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	_ = json.NewEncoder(w).Encode(healthResponse{
		Status:   status,
		Database: dbStatus,
		Version:  s.Version,
		Time:     time.Now().UTC().Format(time.RFC3339),
	})
}

func pingDatabase(ctx context.Context, s *Server) error {
	if s.Pool == nil {
		return nil
	}
	return s.Pool.Ping(ctx)
}
