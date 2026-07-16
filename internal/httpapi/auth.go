package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/yixian-huang/navax/internal/auth"
)

const sessionCookieName = "navax_session"

type AuthHandler struct {
	service       *auth.Service
	secureCookies bool
	instanceName  string
	publicBaseURL string
	version       string
	mailer        Mailer
}

type AuthHandlerOptions struct {
	SecureCookies bool
	InstanceName  string
	PublicBaseURL string
	Version       string
	Mailer        Mailer
}

func NewAuthHandler(service *auth.Service, options AuthHandlerOptions) *AuthHandler {
	return &AuthHandler{
		service: service, secureCookies: options.SecureCookies, instanceName: options.InstanceName,
		publicBaseURL: options.PublicBaseURL, version: options.Version, mailer: options.Mailer,
	}
}

func (h *AuthHandler) Mount(router chi.Router) {
	router.Get("/bootstrap/status", h.bootstrapStatus)
	router.Post("/bootstrap", h.bootstrap)
	router.Get("/auth/session", h.currentSession)
	router.Post("/auth/login", h.login)
	router.Post("/auth/logout", h.logout)
	router.Post("/auth/password/forgot", h.forgotPassword)
	router.Post("/auth/password/reset", h.resetPassword)
	router.Get("/auth/invitations/{token}", h.validateInvitation)
	router.Post("/auth/invitations/{token}/register", h.register)
}

type bootstrapRequest struct {
	AdminUsername string `json:"adminUsername"`
	AdminEmail    string `json:"adminEmail"`
	AdminPassword string `json:"adminPassword"`
	InstanceName  string `json:"instanceName"`
	PublicBaseURL string `json:"publicBaseUrl"`
}

func (h *AuthHandler) bootstrapStatus(w http.ResponseWriter, r *http.Request) {
	initialized, err := h.service.Initialized(r.Context())
	if err != nil {
		WriteError(w, r, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "数据库暂不可用", nil)
		return
	}
	WriteJSON(w, r, http.StatusOK, map[string]any{
		"initialized":   initialized,
		"setupRequired": !initialized,
		"version":       h.version,
		"instanceName":  h.instanceName,
		"publicBaseUrl": h.publicBaseURL,
	})
}

func (h *AuthHandler) bootstrap(w http.ResponseWriter, r *http.Request) {
	var request bootstrapRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	session, token, err := h.service.Bootstrap(r.Context(), r.Header.Get("X-Setup-Token"), auth.BootstrapInput{
		Username: request.AdminUsername, Email: request.AdminEmail, Password: request.AdminPassword,
		InstanceName: request.InstanceName, PublicBaseURL: request.PublicBaseURL,
	})
	if err != nil {
		h.writeAuthError(w, r, err)
		return
	}
	h.setSessionCookie(w, token, session.ExpiresAt)
	WriteJSON(w, r, http.StatusCreated, authSessionData(session))
}

func (h *AuthHandler) currentSession(w http.ResponseWriter, r *http.Request) {
	token := readSessionCookie(r)
	session, err := h.service.Authenticate(r.Context(), token)
	if err != nil {
		WriteJSON(w, r, http.StatusOK, map[string]any{"authenticated": false, "user": nil, "expiresAt": nil})
		return
	}
	WriteJSON(w, r, http.StatusOK, authSessionData(session))
}

func (h *AuthHandler) login(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	session, token, err := h.service.Login(r.Context(), request.Email, request.Password, deviceSummary(r))
	if err != nil {
		h.writeAuthError(w, r, err)
		return
	}
	h.setSessionCookie(w, token, session.ExpiresAt)
	WriteJSON(w, r, http.StatusOK, authSessionData(session))
}

// forgotPassword starts self-service recovery. It always responds identically so
// the endpoint cannot be used to probe which emails are registered; a reset link
// is emailed only when the account exists and an SMTP provider is configured.
func (h *AuthHandler) forgotPassword(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Email string `json:"email"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	result, err := h.service.RequestPasswordReset(r.Context(), request.Email)
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "服务器内部错误", nil)
		return
	}
	if result.Sent && h.mailer != nil && h.mailer.MailConfigured(r.Context()) {
		resetURL := strings.TrimRight(h.publicBaseURL, "/") + "/reset-password?token=" + url.QueryEscape(result.Token)
		message := passwordResetMessage(h.instanceName, result.User.Email, resetURL, result.ExpiresAt)
		if sendErr := h.mailer.SendMail(r.Context(), message); sendErr != nil {
			slog.Warn("send password reset email", "error", sendErr)
		}
	}
	WriteJSON(w, r, http.StatusOK, map[string]any{"message": "如果该邮箱对应有效账号，我们已发送密码重置邮件。"})
}

// resetPassword completes recovery by consuming a reset token and setting a new
// password. The token machinery revokes every existing session for the account.
func (h *AuthHandler) resetPassword(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	if err := h.service.ResetPassword(r.Context(), request.Token, request.Password); err != nil {
		h.writeAuthError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, map[string]any{"message": "密码已重置，请使用新密码登录。"})
}

func (h *AuthHandler) logout(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Logout(r.Context(), readSessionCookie(r)); err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "退出失败", nil)
		return
	}
	h.clearSessionCookie(w)
	WriteJSON(w, r, http.StatusOK, nil)
}

func (h *AuthHandler) validateInvitation(w http.ResponseWriter, r *http.Request) {
	info, err := h.service.ValidateInvitation(r.Context(), chi.URLParam(r, "token"))
	if err != nil {
		h.writeAuthError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, map[string]any{
		"valid": true, "inviterName": info.InviterName, "expiresAt": info.ExpiresAt,
	})
}

func (h *AuthHandler) register(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	session, token, err := h.service.Register(r.Context(), chi.URLParam(r, "token"), auth.RegisterInput{
		Username: request.Username, Email: request.Email, Password: request.Password, Device: deviceSummary(r),
	})
	if err != nil {
		h.writeAuthError(w, r, err)
		return
	}
	h.setSessionCookie(w, token, session.ExpiresAt)
	WriteJSON(w, r, http.StatusCreated, authSessionData(session))
}

func (h *AuthHandler) writeAuthError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, auth.ErrTooManyAttempts):
		var throttled *auth.ThrottledError
		if errors.As(err, &throttled) {
			w.Header().Set("Retry-After", strconv.Itoa(max(1, int(throttled.RetryAfter.Seconds()))))
		}
		WriteError(w, r, http.StatusTooManyRequests, "RATE_LIMITED", "登录尝试过于频繁，请稍后再试", nil)
	case errors.Is(err, auth.ErrInvalidCredentials):
		WriteError(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "邮箱或密码错误", nil)
	case errors.Is(err, auth.ErrAccountDisabled):
		WriteError(w, r, http.StatusForbidden, "ACCOUNT_DISABLED", "账号已被禁用", nil)
	case errors.Is(err, auth.ErrInvalidSetupToken):
		WriteError(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "初始化令牌无效", nil)
	case errors.Is(err, auth.ErrAlreadyInitialized):
		WriteError(w, r, http.StatusConflict, "INSTANCE_ALREADY_INITIALIZED", "实例已完成初始化", nil)
	case errors.Is(err, auth.ErrInvitationExpired):
		WriteError(w, r, http.StatusGone, "INVITATION_EXPIRED", "邀请已过期", nil)
	case errors.Is(err, auth.ErrInvitationExhausted):
		WriteError(w, r, http.StatusGone, "INVITATION_EXHAUSTED", "邀请使用次数已耗尽", nil)
	case errors.Is(err, auth.ErrInvitationInvalid):
		WriteError(w, r, http.StatusNotFound, "INVITATION_NOT_FOUND", "邀请不存在", nil)
	case errors.Is(err, auth.ErrConflict):
		WriteError(w, r, http.StatusConflict, "CONFLICT", "用户名、邮箱或页面地址已存在", nil)
	case errors.Is(err, auth.ErrInvalidResetToken):
		WriteError(w, r, http.StatusBadRequest, "INVALID_RESET_TOKEN", "重置链接无效或已过期", nil)
	case errors.Is(err, auth.ErrInvalidInput):
		WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "用户名、邮箱、密码或实例信息无效", nil)
	default:
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "服务器内部错误", nil)
	}
}

func (h *AuthHandler) setSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: token, Path: "/", Expires: expiresAt,
		MaxAge: int(time.Until(expiresAt).Seconds()), HttpOnly: true, Secure: h.secureCookies, SameSite: http.SameSiteLaxMode,
	})
}

func (h *AuthHandler) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: "", Path: "/", MaxAge: -1, Expires: time.Unix(1, 0),
		HttpOnly: true, Secure: h.secureCookies, SameSite: http.SameSiteLaxMode,
	})
}

func readSessionCookie(r *http.Request) string {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func authSessionData(session auth.Session) map[string]any {
	return map[string]any{
		"authenticated": true,
		"user": map[string]any{
			"id": session.User.ID, "username": session.User.Username, "email": session.User.Email,
			"avatarUrl": session.User.AvatarURL, "bio": session.User.Bio, "role": session.User.Role,
			"status": session.User.Status, "createdAt": session.User.CreatedAt, "updatedAt": session.User.UpdatedAt,
		},
		"expiresAt": session.ExpiresAt,
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		WriteError(w, r, http.StatusBadRequest, "MALFORMED_REQUEST", "请求 JSON 无效", err)
		return false
	}
	if decoder.Decode(&struct{}{}) == nil {
		WriteError(w, r, http.StatusBadRequest, "MALFORMED_REQUEST", "请求只能包含一个 JSON 对象", nil)
		return false
	}
	return true
}

func deviceSummary(r *http.Request) string {
	value := strings.TrimSpace(r.UserAgent())
	if len(value) > 240 {
		value = value[:240]
	}
	return value
}
