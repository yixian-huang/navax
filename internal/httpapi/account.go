package httpapi

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/yixian-huang/navax/internal/auth"
)

type AccountHandler struct {
	service *auth.Service
}

func NewAccountHandler(service *auth.Service) *AccountHandler {
	return &AccountHandler{service: service}
}

func (h *AccountHandler) Mount(router chi.Router) {
	router.Route("/me", func(me chi.Router) {
		me.Use(RequireSession(h.service))
		me.Get("/profile", h.profile)
		me.Patch("/profile", h.updateProfile)
		me.Patch("/password", h.changePassword)
		me.Get("/sessions", h.sessions)
		me.Delete("/sessions/{sessionId}", h.revokeSession)
	})
}

func (h *AccountHandler) profile(w http.ResponseWriter, r *http.Request) {
	session, _ := SessionFromContext(r.Context())
	user, err := h.service.Profile(r.Context(), session.User.ID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, userData(user))
}

func (h *AccountHandler) updateProfile(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Username  *string `json:"username"`
		Bio       *string `json:"bio"`
		AvatarURL *string `json:"avatarUrl"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	session, _ := SessionFromContext(r.Context())
	user, err := h.service.UpdateProfile(r.Context(), session.User.ID, auth.ProfilePatch{
		Username: request.Username, Bio: request.Bio, AvatarURL: request.AvatarURL,
	})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, userData(user))
}

func (h *AccountHandler) changePassword(w http.ResponseWriter, r *http.Request) {
	var request struct {
		CurrentPassword     string `json:"currentPassword"`
		NewPassword         string `json:"newPassword"`
		RevokeOtherSessions *bool  `json:"revokeOtherSessions"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	revokeOthers := true
	if request.RevokeOtherSessions != nil {
		revokeOthers = *request.RevokeOtherSessions
	}
	session, _ := SessionFromContext(r.Context())
	err := h.service.ChangePassword(
		r.Context(), session.User.ID, session.ID,
		request.CurrentPassword, request.NewPassword, revokeOthers,
	)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, nil)
}

func (h *AccountHandler) sessions(w http.ResponseWriter, r *http.Request) {
	session, _ := SessionFromContext(r.Context())
	items, err := h.service.Sessions(r.Context(), session.User.ID, session.ID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	data := make([]map[string]any, 0, len(items))
	for _, item := range items {
		data = append(data, map[string]any{
			"id": item.ID, "current": item.Current, "device": item.Device,
			"approximateLocation": item.ApproximateLocation, "createdAt": item.CreatedAt,
			"lastSeenAt": item.LastSeenAt, "expiresAt": item.ExpiresAt,
		})
	}
	WriteJSON(w, r, http.StatusOK, data)
}

func (h *AccountHandler) revokeSession(w http.ResponseWriter, r *http.Request) {
	session, _ := SessionFromContext(r.Context())
	if err := h.service.RevokeSession(r.Context(), session.User.ID, chi.URLParam(r, "sessionId")); err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, nil)
}

func (h *AccountHandler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalidInput):
		WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "账号信息格式无效", nil)
	case errors.Is(err, auth.ErrCurrentPassword), errors.Is(err, auth.ErrInvalidCredentials):
		WriteError(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "当前密码错误", nil)
	case errors.Is(err, auth.ErrConflict):
		WriteError(w, r, http.StatusConflict, "CONFLICT", "用户名已被使用", nil)
	case errors.Is(err, auth.ErrNotFound):
		WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "账号或会话不存在", nil)
	default:
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "账号操作失败", nil)
	}
}

func userData(user auth.User) map[string]any {
	return map[string]any{
		"id": user.ID, "username": user.Username, "email": user.Email,
		"avatarUrl": user.AvatarURL, "bio": user.Bio, "role": user.Role,
		"status": user.Status, "createdAt": user.CreatedAt, "updatedAt": user.UpdatedAt,
	}
}
