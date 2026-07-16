package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/yixian-huang/navax/internal/auth"
	"github.com/yixian-huang/navax/internal/maintenance"
)

type BackupHandler struct {
	service        *maintenance.BackupService
	auth           *auth.Service
	requestRestart func()
}

type BackupHandlerOptions struct {
	Auth           *auth.Service
	RequestRestart func()
}

func NewBackupHandler(service *maintenance.BackupService, options ...BackupHandlerOptions) *BackupHandler {
	handler := &BackupHandler{service: service}
	if len(options) > 0 {
		handler.auth = options[0].Auth
		handler.requestRestart = options[0].RequestRestart
	}
	return handler
}

func (h *BackupHandler) Mount(router chi.Router) {
	router.Get("/backups", h.list)
	router.Post("/backups", h.create)
	router.Get("/backups/{backupId}", h.download)
	router.Post("/backups/{backupId}/restore-token", h.restoreToken)
	router.Post("/backups/{backupId}/restore", h.restore)
}

func (h *BackupHandler) restoreToken(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	if h.auth == nil {
		WriteError(w, r, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "恢复服务未配置", nil)
		return
	}
	session, _ := SessionFromContext(r.Context())
	if err := h.auth.VerifyCurrentPassword(r.Context(), session.User.ID, request.Password); err != nil {
		WriteError(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "密码验证失败", nil)
		return
	}
	token, err := h.service.CreateRestoreToken(r.Context(), chi.URLParam(r, "backupId"), session.User.ID)
	if err != nil {
		WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "备份不存在", nil)
		return
	}
	WriteJSON(w, r, http.StatusOK, token)
}

func (h *BackupHandler) restore(w http.ResponseWriter, r *http.Request) {
	var request struct {
		RestoreToken string `json:"restoreToken"`
		Confirmation string `json:"confirmation"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	if request.Confirmation != "RESTORE_BACKUP" {
		WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "恢复确认文本无效", nil)
		return
	}
	session, _ := SessionFromContext(r.Context())
	err := h.service.StageRestore(r.Context(), chi.URLParam(r, "backupId"), session.User.ID, request.RestoreToken)
	switch {
	case err == nil:
		WriteJSON(w, r, http.StatusAccepted, nil)
		if h.requestRestart != nil {
			h.requestRestart()
		}
	case errors.Is(err, maintenance.ErrRestoreToken):
		WriteError(w, r, http.StatusConflict, "CONFLICT", "恢复令牌无效、已使用或已过期", nil)
	case errors.Is(err, maintenance.ErrBackupInvalid):
		WriteError(w, r, http.StatusConflict, "CONFLICT", "备份校验失败", nil)
	case errors.Is(err, maintenance.ErrRestoreNotConfigured):
		WriteError(w, r, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "恢复服务未配置", nil)
	default:
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "暂存恢复失败", nil)
	}
}

func (h *BackupHandler) list(w http.ResponseWriter, r *http.Request) {
	backups, err := h.service.List(r.Context())
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "读取备份列表失败", nil)
		return
	}
	WriteJSON(w, r, http.StatusOK, backups)
}

func (h *BackupHandler) create(w http.ResponseWriter, r *http.Request) {
	session, _ := SessionFromContext(r.Context())
	backup, err := h.service.Create(r.Context(), "manual", session.User.ID)
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "创建备份失败", nil)
		return
	}
	WriteJSON(w, r, http.StatusCreated, backup)
}

func (h *BackupHandler) download(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "backupId")
	path, err := h.service.Path(r.Context(), id)
	if err != nil {
		WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "备份不存在", nil)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	extension := filepath.Ext(path)
	if extension == "" {
		extension = ".navbak"
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="navax-backup-%s%s"`, id, extension))
	http.ServeFile(w, r, path)
}
