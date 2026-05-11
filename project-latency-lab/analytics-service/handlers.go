package main

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

type NoteStats struct {
	NoteID       string   `json:"note_id"`
	ViewCount    int64    `json:"view_count"`
	RecentViews  []string `json:"recent_views"`
}

func handleNoteStats(store *Store, cache *Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		applyDelay(ctx, StageHandler)

		id := strings.TrimPrefix(r.URL.Path, "/api/analytics/")
		if id == "" || id == "top" {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "note id required"})
			return
		}

		if shouldFail(ctx) {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "injected failure"})
			return
		}

		// Try cache first.
		count, recent, hit := cache.GetStats(ctx, id)
		if !hit {
			c, err := store.GetViewCount(ctx, id)
			if err != nil {
				if err == sql.ErrNoRows {
					writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "not found"})
					return
				}
				slog.Error("get view count", "id", id, "error", err)
				writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal error"})
				return
			}
			rs, err := store.RecentViews(ctx, id, 50)
			if err != nil {
				slog.Warn("recent views", "id", id, "error", err)
			}
			count = c
			recent = rs
			cache.SetStats(ctx, id, count, recent)
		}

		if recent == nil {
			recent = []string{}
		}
		writeJSON(w, http.StatusOK, NoteStats{
			NoteID:      id,
			ViewCount:   count,
			RecentViews: recent,
		})
	}
}

func handleTop(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		applyDelay(ctx, StageHandler)

		if shouldFail(ctx) {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "injected failure"})
			return
		}

		rows, err := store.TopNotes(ctx, 10)
		if err != nil {
			slog.Error("top notes", "error", err)
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal error"})
			return
		}
		if rows == nil {
			rows = []TopNote{}
		}
		writeJSON(w, http.StatusOK, rows)
	}
}

func handleHealth(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := store.db.Ping(); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unhealthy", "error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
