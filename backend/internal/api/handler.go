package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/eterealink/eterealink/backend/internal/domain"
	"github.com/eterealink/eterealink/backend/internal/service"
)

type ReadinessChecker interface {
	Ping(ctx context.Context) error
}

type Handler struct {
	transfers *service.Transfers
	readiness ReadinessChecker
	logger    *slog.Logger
}

func NewHandler(transfers *service.Transfers, readiness ReadinessChecker, logger *slog.Logger) http.Handler {
	handler := &Handler{transfers: transfers, readiness: readiness, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handler.health)
	mux.HandleFunc("GET /readyz", handler.ready)
	mux.HandleFunc("POST /v1/uploads", handler.createAnonymousUpload)
	mux.HandleFunc("POST /v1/uploads/{id}/complete", handler.completeUpload)
	mux.HandleFunc("GET /v1/shares/{code}", handler.resolveShare)
	return requestLog(logger, recoverPanic(logger, mux))
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := h.readiness.Ping(ctx); err != nil {
		h.logger.Warn("readiness check failed", "error", err)
		writeError(w, http.StatusServiceUnavailable, "not_ready", "service dependencies are unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (h *Handler) createAnonymousUpload(w http.ResponseWriter, r *http.Request) {
	var input service.CreateAnonymousUploadInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	result, err := h.transfers.CreateAnonymousUpload(r.Context(), input)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidName), errors.Is(err, service.ErrInvalidSize):
			writeError(w, http.StatusUnprocessableEntity, "validation_failed", err.Error())
		default:
			h.logger.Error("create upload failed", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "unable to create upload")
		}
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) completeUpload(w http.ResponseWriter, r *http.Request) {
	fileID := strings.TrimSpace(r.PathValue("id"))
	if fileID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "file id is required")
		return
	}

	file, err := h.transfers.CompleteUpload(r.Context(), fileID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "upload was not found")
			return
		}
		h.logger.Error("complete upload failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "unable to complete upload")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"file": file})
}

func (h *Handler) resolveShare(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimSpace(r.PathValue("code"))
	if code == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "share code is required")
		return
	}

	result, err := h.transfers.ResolveShare(r.Context(), code)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "share was not found")
		case errors.Is(err, domain.ErrExpired):
			writeError(w, http.StatusGone, "expired", "share has expired")
		case errors.Is(err, domain.ErrRevoked):
			writeError(w, http.StatusGone, "revoked", "share has been revoked")
		case errors.Is(err, domain.ErrConflict):
			writeError(w, http.StatusConflict, "upload_incomplete", "file upload is not complete")
		default:
			h.logger.Error("resolve share failed", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "unable to resolve share")
		}
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return errors.New("request body must be valid JSON with only supported fields")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain a single JSON object")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func requestLog(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("request", "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(started).Milliseconds())
	})
}

func recoverPanic(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("panic recovered", "value", recovered)
				writeError(w, http.StatusInternalServerError, "internal_error", "unexpected server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
