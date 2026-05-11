package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
)

type CreateRequest struct {
	Author string `json:"author"`
	Body   string `json:"body"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func handleCreate(store *Store, cache *Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		applyDelay(ctx, StageHandler)

		var req CreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
			return
		}
		if req.Author == "" || req.Body == "" {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "author and body are required"})
			return
		}
		if len(req.Body) > 10_000 {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "body too long (max 10000 chars)"})
			return
		}

		if shouldFail(ctx) {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "injected failure"})
			return
		}

		id := generateID()
		n, err := store.InsertNote(ctx, id, req.Author, req.Body)
		if err != nil {
			slog.Error("insert note", "id", id, "error", err)
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "failed to create note"})
			return
		}
		cache.SetNote(ctx, n)

		writeJSON(w, http.StatusCreated, n)
	}
}

func handleGet(store *Store, cache *Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		applyDelay(ctx, StageHandler)

		id := strings.TrimPrefix(r.URL.Path, "/api/notes/")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "id required"})
			return
		}

		if shouldFail(ctx) {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "injected failure"})
			return
		}

		// Try cache first.
		n, err := cache.GetNote(ctx, id)
		if err != nil {
			n, err = store.GetNote(ctx, id)
			if err != nil {
				if err == sql.ErrNoRows {
					writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "not found"})
					return
				}
				slog.Error("get note", "id", id, "error", err)
				writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal error"})
				return
			}
			cache.SetNote(ctx, n)
		}

		// Best-effort view bump (in-band, so latency knobs apply to it too).
		if err := store.RecordView(ctx, id); err != nil {
			slog.Warn("record view", "id", id, "error", err)
		}
		cache.IncrViewCount(ctx, id)

		writeJSON(w, http.StatusOK, n)
	}
}

func handleList(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		applyDelay(ctx, StageHandler)

		if shouldFail(ctx) {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "injected failure"})
			return
		}

		author := r.URL.Query().Get("author")
		notes, err := store.ListNotes(ctx, author)
		if err != nil {
			slog.Error("list notes", "author", author, "error", err)
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal error"})
			return
		}
		if notes == nil {
			notes = []Note{}
		}
		writeJSON(w, http.StatusOK, notes)
	}
}

func handleDelete(store *Store, cache *Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		applyDelay(ctx, StageHandler)

		id := strings.TrimPrefix(r.URL.Path, "/api/notes/")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "id required"})
			return
		}

		if shouldFail(ctx) {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "injected failure"})
			return
		}

		if err := store.DeleteNote(ctx, id); err != nil {
			if err == sql.ErrNoRows {
				writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "not found"})
				return
			}
			slog.Error("delete note", "id", id, "error", err)
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal error"})
			return
		}
		cache.DeleteNote(ctx, id)
		w.WriteHeader(http.StatusNoContent)
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

func generateID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}
