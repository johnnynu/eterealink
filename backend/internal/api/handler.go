package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/eterealink/eterealink/backend/internal/domain"
	"github.com/eterealink/eterealink/backend/internal/identity"
	"github.com/eterealink/eterealink/backend/internal/service"
)

type ReadinessChecker interface {
	Ping(ctx context.Context) error
}

type Handler struct {
	transfers *service.Transfers
	bundles   *service.Bundles
	files     *service.Files
	folders   *service.Folders
	users     *service.Users
	verifier  identity.Verifier
	readiness ReadinessChecker
	logger    *slog.Logger
}

func NewHandler(
	transfers *service.Transfers,
	bundles *service.Bundles,
	files *service.Files,
	users *service.Users,
	verifier identity.Verifier,
	readiness ReadinessChecker,
	logger *slog.Logger,
	folders ...*service.Folders,
) http.Handler {
	handler := &Handler{
		transfers: transfers, bundles: bundles, files: files, users: users, verifier: verifier,
		readiness: readiness, logger: logger,
	}
	if len(folders) > 0 {
		handler.folders = folders[0]
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handler.health)
	mux.HandleFunc("GET /readyz", handler.ready)
	mux.Handle("GET /v1/me", handler.requireAuthentication(http.HandlerFunc(handler.me)))
	mux.Handle("POST /v1/files", handler.requireAuthentication(http.HandlerFunc(handler.createPersistentFile)))
	mux.Handle("GET /v1/files", handler.requireAuthentication(http.HandlerFunc(handler.listPersistentFiles)))
	mux.Handle("POST /v1/files/{id}/complete", handler.requireAuthentication(http.HandlerFunc(handler.completePersistentFile)))
	mux.Handle("GET /v1/files/{id}/download", handler.requireAuthentication(http.HandlerFunc(handler.downloadPersistentFile)))
	mux.Handle("POST /v1/files/{id}/shares", handler.requireAuthentication(http.HandlerFunc(handler.createPersistentFileShare)))
	mux.Handle("DELETE /v1/files/{id}/shares/{shareID}", handler.requireAuthentication(http.HandlerFunc(handler.revokePersistentFileShare)))
	mux.Handle("DELETE /v1/files/{id}", handler.requireAuthentication(http.HandlerFunc(handler.deletePersistentFile)))
	mux.Handle("PATCH /v1/files/move", handler.requireAuthentication(http.HandlerFunc(handler.movePersistentFiles)))
	mux.Handle("POST /v1/folders", handler.requireAuthentication(http.HandlerFunc(handler.createFolder)))
	mux.Handle("GET /v1/folders", handler.requireAuthentication(http.HandlerFunc(handler.listRootFolders)))
	mux.Handle("GET /v1/folders/{id}", handler.requireAuthentication(http.HandlerFunc(handler.getFolder)))
	mux.Handle("PATCH /v1/folders/{id}", handler.requireAuthentication(http.HandlerFunc(handler.updateFolder)))
	mux.Handle("DELETE /v1/folders/{id}", handler.requireAuthentication(http.HandlerFunc(handler.deleteFolder)))
	mux.Handle("GET /v1/folders/{id}/members", handler.requireAuthentication(http.HandlerFunc(handler.listFolderMembers)))
	mux.Handle("POST /v1/folders/{id}/members", handler.requireAuthentication(http.HandlerFunc(handler.addFolderMember)))
	mux.Handle("DELETE /v1/folders/{id}/members/{userID}", handler.requireAuthentication(http.HandlerFunc(handler.removeFolderMember)))
	mux.HandleFunc("POST /v1/uploads", handler.createAnonymousUpload)
	mux.HandleFunc("POST /v1/uploads/{id}/complete", handler.completeUpload)
	mux.HandleFunc("POST /v1/transfers", handler.createAnonymousTransfer)
	mux.HandleFunc("POST /v1/transfers/{transferID}/files/{fileID}/complete", handler.completeTransferFile)
	mux.HandleFunc("GET /v1/shares/{code}", handler.resolveShare)
	return requestLog(logger, recoverPanic(logger, mux))
}

type authenticatedUserContextKey struct{}

func (h *Handler) requireAuthentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.verifier == nil || h.users == nil {
			writeError(w, http.StatusServiceUnavailable, "authentication_unavailable", "authentication is not configured")
			return
		}
		rawToken, ok := bearerToken(r.Header.Get("Authorization"))
		if !ok {
			writeAuthenticationRequired(w, "a valid bearer token is required")
			return
		}
		claims, err := h.verifier.VerifyIDToken(r.Context(), rawToken)
		if err != nil {
			h.logger.Warn("firebase token verification failed", "error", err)
			writeAuthenticationRequired(w, "a valid bearer token is required")
			return
		}
		user, err := h.users.Provision(r.Context(), service.AuthenticatedIdentity{
			FirebaseUID: claims.UID,
			Email:       claims.Email,
			DisplayName: claims.DisplayName,
		})
		if errors.Is(err, service.ErrInvalidIdentity) {
			writeAuthenticationRequired(w, "the authenticated identity is incomplete")
			return
		}
		if err != nil {
			h.logger.Error("provision authenticated user failed", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "unable to provision user")
			return
		}
		ctx := context.WithValue(r.Context(), authenticatedUserContextKey{}, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func bearerToken(value string) (string, bool) {
	parts := strings.Fields(value)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}

func writeAuthenticationRequired(w http.ResponseWriter, message string) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="eterealink"`)
	writeError(w, http.StatusUnauthorized, "unauthenticated", message)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeAuthenticationRequired(w, "a valid bearer token is required")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func authenticatedUser(r *http.Request) (domain.User, bool) {
	user, ok := r.Context().Value(authenticatedUserContextKey{}).(domain.User)
	return user, ok
}

func (h *Handler) createPersistentFile(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeAuthenticationRequired(w, "a valid bearer token is required")
		return
	}
	var input service.CreateFileUploadInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	result, err := h.files.CreateUpload(r.Context(), user.ID, input)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidName), errors.Is(err, service.ErrInvalidSize):
			writeError(w, http.StatusUnprocessableEntity, "validation_failed", err.Error())
		case errors.Is(err, domain.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "destination folder was not found")
		case errors.Is(err, domain.ErrConflict):
			writeError(w, http.StatusConflict, "name_conflict", "a file with this name already exists in the folder")
		case errors.Is(err, service.ErrStorageQuotaExceeded):
			writeError(w, http.StatusConflict, "storage_quota_exceeded", err.Error())
		default:
			h.logger.Error("create persistent upload failed", "user_id", user.ID, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "unable to create persistent upload")
		}
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) listPersistentFiles(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeAuthenticationRequired(w, "a valid bearer token is required")
		return
	}
	library, err := h.files.List(r.Context(), user.ID)
	if err != nil {
		h.logger.Error("list persistent files failed", "user_id", user.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "unable to list files")
		return
	}
	writeJSON(w, http.StatusOK, library)
}

func (h *Handler) completePersistentFile(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeAuthenticationRequired(w, "a valid bearer token is required")
		return
	}
	fileID := strings.TrimSpace(r.PathValue("id"))
	if fileID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "file id is required")
		return
	}
	file, err := h.files.CompleteUpload(r.Context(), user.ID, fileID)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "file upload was not found")
		case errors.Is(err, service.ErrUploadObjectMissing):
			writeError(w, http.StatusConflict, "upload_missing", "uploaded object was not found")
		case errors.Is(err, service.ErrUploadObjectMismatch):
			writeError(w, http.StatusConflict, "upload_mismatch", "uploaded object does not match declared metadata")
		default:
			h.logger.Error("complete persistent upload failed", "user_id", user.ID, "file_id", fileID, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "unable to complete persistent upload")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"file": file})
}

func (h *Handler) downloadPersistentFile(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeAuthenticationRequired(w, "a valid bearer token is required")
		return
	}
	fileID := strings.TrimSpace(r.PathValue("id"))
	result, err := h.files.Download(r.Context(), user.ID, fileID)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "file was not found")
		case errors.Is(err, domain.ErrConflict):
			writeError(w, http.StatusConflict, "upload_incomplete", "file upload is not complete")
		default:
			h.logger.Error("create persistent download failed", "user_id", user.ID, "file_id", fileID, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "unable to prepare file download")
		}
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) createPersistentFileShare(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeAuthenticationRequired(w, "a valid bearer token is required")
		return
	}
	fileID := strings.TrimSpace(r.PathValue("id"))
	if fileID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "file id is required")
		return
	}
	var input service.CreateFileShareInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	result, err := h.files.CreateShare(r.Context(), user.ID, fileID, input)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidShareExpiration):
			writeError(w, http.StatusUnprocessableEntity, "validation_failed", err.Error())
		case errors.Is(err, domain.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "file was not found")
		case errors.Is(err, domain.ErrConflict):
			writeError(w, http.StatusConflict, "share_conflict", "file is not ready or already has an active link")
		default:
			h.logger.Error("create persistent file share failed", "user_id", user.ID, "file_id", fileID, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "unable to create share link")
		}
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) revokePersistentFileShare(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeAuthenticationRequired(w, "a valid bearer token is required")
		return
	}
	fileID := strings.TrimSpace(r.PathValue("id"))
	shareID := strings.TrimSpace(r.PathValue("shareID"))
	if fileID == "" || shareID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "file id and share id are required")
		return
	}
	if err := h.files.RevokeShare(r.Context(), user.ID, fileID, shareID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "active share link was not found")
			return
		}
		h.logger.Error("revoke persistent file share failed", "user_id", user.ID, "file_id", fileID, "share_id", shareID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "unable to revoke share link")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) deletePersistentFile(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeAuthenticationRequired(w, "a valid bearer token is required")
		return
	}
	fileID := strings.TrimSpace(r.PathValue("id"))
	if err := h.files.Delete(r.Context(), user.ID, fileID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "file was not found")
			return
		}
		h.logger.Error("delete persistent file failed", "user_id", user.ID, "file_id", fileID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "unable to delete file")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) folderService(w http.ResponseWriter) bool {
	if h.folders == nil {
		writeError(w, http.StatusServiceUnavailable, "folders_unavailable", "folder service is not configured")
		return false
	}
	return true
}

func folderRequestUser(w http.ResponseWriter, r *http.Request) (domain.User, bool) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeAuthenticationRequired(w, "a valid bearer token is required")
	}
	return user, ok
}

func (h *Handler) createFolder(w http.ResponseWriter, r *http.Request) {
	user, ok := folderRequestUser(w, r)
	if !ok || !h.folderService(w) {
		return
	}
	var input service.CreateFolderInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	folder, err := h.folders.Create(r.Context(), user.ID, input)
	if h.writeFolderError(w, err, "create") {
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"folder": folder})
}

func (h *Handler) listRootFolders(w http.ResponseWriter, r *http.Request) {
	user, ok := folderRequestUser(w, r)
	if !ok || !h.folderService(w) {
		return
	}
	result, err := h.folders.ListRoot(r.Context(), user.ID, r.URL.Query().Get("scope"), folderListInput(r))
	if h.writeFolderError(w, err, "list") {
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) getFolder(w http.ResponseWriter, r *http.Request) {
	user, ok := folderRequestUser(w, r)
	if !ok || !h.folderService(w) {
		return
	}
	result, err := h.folders.Contents(r.Context(), user.ID, r.PathValue("id"), folderListInput(r))
	if h.writeFolderError(w, err, "read") {
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func folderListInput(r *http.Request) service.ListFolderInput {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	return service.ListFolderInput{
		Search: r.URL.Query().Get("q"), Sort: r.URL.Query().Get("sort"), Filter: r.URL.Query().Get("filter"),
		Limit: limit, Cursor: r.URL.Query().Get("cursor"),
	}
}

func (h *Handler) updateFolder(w http.ResponseWriter, r *http.Request) {
	user, ok := folderRequestUser(w, r)
	if !ok || !h.folderService(w) {
		return
	}
	var input service.UpdateFolderInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	folder, err := h.folders.Update(r.Context(), user.ID, r.PathValue("id"), input)
	if h.writeFolderError(w, err, "update") {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"folder": folder})
}

func (h *Handler) deleteFolder(w http.ResponseWriter, r *http.Request) {
	user, ok := folderRequestUser(w, r)
	if !ok || !h.folderService(w) {
		return
	}
	if h.writeFolderError(w, h.folders.Delete(r.Context(), user.ID, r.PathValue("id")), "delete") {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listFolderMembers(w http.ResponseWriter, r *http.Request) {
	user, ok := folderRequestUser(w, r)
	if !ok || !h.folderService(w) {
		return
	}
	members, err := h.folders.Members(r.Context(), user.ID, r.PathValue("id"))
	if h.writeFolderError(w, err, "list members") {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": members})
}

func (h *Handler) addFolderMember(w http.ResponseWriter, r *http.Request) {
	user, ok := folderRequestUser(w, r)
	if !ok || !h.folderService(w) {
		return
	}
	var input service.AddFolderMemberInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	member, err := h.folders.AddMember(r.Context(), user.ID, r.PathValue("id"), input)
	if h.writeFolderError(w, err, "share") {
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"member": member})
}

func (h *Handler) removeFolderMember(w http.ResponseWriter, r *http.Request) {
	user, ok := folderRequestUser(w, r)
	if !ok || !h.folderService(w) {
		return
	}
	err := h.folders.RemoveMember(r.Context(), user.ID, r.PathValue("id"), r.PathValue("userID"))
	if h.writeFolderError(w, err, "remove member") {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) movePersistentFiles(w http.ResponseWriter, r *http.Request) {
	user, ok := folderRequestUser(w, r)
	if !ok || !h.folderService(w) {
		return
	}
	var input service.MoveFilesInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	err := h.folders.MoveFiles(r.Context(), user.ID, input)
	if h.writeFolderError(w, err, "move files") {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) writeFolderError(w http.ResponseWriter, err error, operation string) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, service.ErrInvalidFolderName), errors.Is(err, service.ErrInvalidMember), errors.Is(err, service.ErrTooManyFiles), errors.Is(err, service.ErrInvalidLibraryQuery):
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", err.Error())
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "folder or user was not found")
	case errors.Is(err, domain.ErrConflict), errors.Is(err, service.ErrFolderNotEmpty), errors.Is(err, service.ErrInvalidFolderMove):
		writeError(w, http.StatusConflict, "folder_conflict", "the folder operation conflicts with its current contents or hierarchy")
	default:
		h.logger.Error("folder operation failed", "operation", operation, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "unable to "+operation+" folder")
	}
	return true
}

func (h *Handler) createAnonymousTransfer(w http.ResponseWriter, r *http.Request) {
	var input service.CreateAnonymousTransferInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	result, err := h.bundles.CreateAnonymousTransfer(r.Context(), input)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidName), errors.Is(err, service.ErrInvalidSize),
			errors.Is(err, service.ErrInvalidFileCount), errors.Is(err, service.ErrTransferTooLarge),
			errors.Is(err, service.ErrDuplicateFileName):
			writeError(w, http.StatusUnprocessableEntity, "validation_failed", err.Error())
		default:
			h.logger.Error("create transfer failed", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "unable to create transfer")
		}
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) completeTransferFile(w http.ResponseWriter, r *http.Request) {
	transferID := strings.TrimSpace(r.PathValue("transferID"))
	fileID := strings.TrimSpace(r.PathValue("fileID"))
	if transferID == "" || fileID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "transfer id and file id are required")
		return
	}
	file, transfer, err := h.bundles.CompleteFile(r.Context(), transferID, fileID)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "transfer upload was not found")
		case errors.Is(err, service.ErrUploadObjectMissing):
			writeError(w, http.StatusConflict, "upload_missing", "uploaded object was not found")
		case errors.Is(err, service.ErrUploadObjectMismatch):
			writeError(w, http.StatusConflict, "upload_mismatch", "uploaded object does not match declared metadata")
		default:
			h.logger.Error("complete transfer file failed", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "unable to complete transfer file")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"file": file, "transfer": transfer})
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
		switch {
		case errors.Is(err, domain.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "upload was not found")
		case errors.Is(err, service.ErrUploadObjectMissing):
			writeError(w, http.StatusConflict, "upload_missing", "uploaded object was not found")
		case errors.Is(err, service.ErrUploadObjectMismatch):
			writeError(w, http.StatusConflict, "upload_mismatch", "uploaded object does not match declared metadata")
		default:
			h.logger.Error("complete upload failed", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "unable to complete upload")
		}
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
	if errors.Is(err, domain.ErrNotFound) && h.bundles != nil {
		bundle, bundleErr := h.bundles.ResolveShare(r.Context(), code)
		if bundleErr == nil {
			writeJSON(w, http.StatusOK, bundle)
			return
		}
		err = bundleErr
	}
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
		case errors.Is(err, service.ErrTransferNotReady):
			writeError(w, http.StatusConflict, "upload_incomplete", "transfer upload is not complete")
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
