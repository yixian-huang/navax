package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/yixian-huang/navax/internal/idempotency"
	"github.com/yixian-huang/navax/internal/maintenance"
)

type UpdateHandler struct {
	service        *maintenance.UpdateService
	idempotency    *idempotency.Service
	requestRestart func()
}

type UpdateHandlerOptions struct {
	Idempotency    *idempotency.Service
	RequestRestart func()
}

func NewUpdateHandler(service *maintenance.UpdateService, options ...UpdateHandlerOptions) *UpdateHandler {
	handler := &UpdateHandler{service: service}
	if len(options) > 0 {
		handler.idempotency = options[0].Idempotency
		handler.requestRestart = options[0].RequestRestart
	}
	return handler
}

func (h *UpdateHandler) Mount(router chi.Router) {
	router.Get("/update", h.state)
	router.Patch("/update", h.settings)
	router.Post("/update/check", h.check)
	router.Post("/update/apply", h.apply)
}

func (h *UpdateHandler) state(w http.ResponseWriter, r *http.Request) {
	state, err := h.service.State(r.Context())
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "读取更新状态失败", nil)
		return
	}
	WriteJSON(w, r, http.StatusOK, state)
}

func (h *UpdateHandler) settings(w http.ResponseWriter, r *http.Request) {
	var request struct {
		AutoCheck         *bool           `json:"autoCheck"`
		AutoApply         *bool           `json:"autoApply"`
		MaintenanceWindow json.RawMessage `json:"maintenanceWindow"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	var maintenanceWindow *string
	if len(request.MaintenanceWindow) > 0 {
		value := ""
		if string(request.MaintenanceWindow) != "null" {
			if err := json.Unmarshal(request.MaintenanceWindow, &value); err != nil {
				WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "维护窗口格式无效", nil)
				return
			}
		}
		maintenanceWindow = &value
	}
	if request.AutoCheck == nil && request.AutoApply == nil && maintenanceWindow == nil {
		WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "至少提供一个更新设置", nil)
		return
	}
	state, err := h.service.UpdateSettings(r.Context(), request.AutoCheck, request.AutoApply, maintenanceWindow)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, state)
}

func (h *UpdateHandler) check(w http.ResponseWriter, r *http.Request) {
	state, err := h.service.Check(r.Context())
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, state)
}

func (h *UpdateHandler) apply(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Version      string `json:"version"`
		Confirmation string `json:"confirmation"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	if request.Confirmation != "APPLY_UPDATE" {
		WriteError(w, r, http.StatusBadRequest, "VALIDATION_FAILED", "更新确认文本无效", nil)
		return
	}
	session, _ := SessionFromContext(r.Context())
	if h.idempotency == nil {
		WriteError(w, r, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "更新幂等服务未配置", nil)
		return
	}
	reservation, replay, err := h.idempotency.Begin(
		r.Context(), "update:apply", r.Header.Get("Idempotency-Key"), session.User.ID, request,
	)
	if err != nil {
		if errors.Is(err, idempotency.ErrInvalidKey) {
			WriteError(w, r, http.StatusBadRequest, "VALIDATION_FAILED", "Idempotency-Key 必须为 16 至 128 个字符", nil)
		} else {
			WriteError(w, r, http.StatusConflict, "IDEMPOTENCY_CONFLICT", "幂等键正在处理或已用于其他请求", nil)
		}
		return
	}
	if replay != nil {
		var state maintenance.UpdateState
		if err := json.Unmarshal(replay.Data, &state); err != nil {
			WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "读取幂等更新结果失败", nil)
			return
		}
		WriteJSON(w, r, replay.Status, state)
		return
	}
	// Only Abort when Apply itself fails. After binary replacement succeeds,
	// never Abort (replay would re-replace binaries) and always request restart.
	completed := false
	defer func() {
		if !completed {
			reservation.Abort(context.WithoutCancel(r.Context()))
		}
	}()
	state, err := h.service.Apply(r.Context(), request.Version, session.User.ID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	if err := reservation.Complete(r.Context(), http.StatusAccepted, state); err != nil {
		slog.Warn("complete update apply idempotency record", "error", err, "version", request.Version)
	}
	completed = true
	WriteJSON(w, r, http.StatusAccepted, state)
	if state.Status == "restart-required" && h.requestRestart != nil {
		h.requestRestart()
	}
}

func (h *UpdateHandler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, maintenance.ErrUpdateNotConfigured):
		WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "未配置更新清单或签名公钥", nil)
	case errors.Is(err, maintenance.ErrInvalidManifest):
		WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "更新清单签名或内容无效", nil)
	case errors.Is(err, maintenance.ErrContainerManaged):
		WriteError(w, r, http.StatusConflict, "CONFLICT", "容器部署必须由容器编排更新", nil)
	case errors.Is(err, maintenance.ErrUpdateInProgress):
		WriteError(w, r, http.StatusConflict, "CONFLICT", "已有更新正在进行，请稍后重试", nil)
	default:
		WriteError(w, r, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "更新服务暂不可用", nil)
	}
}
