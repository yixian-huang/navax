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
	router.Post("/auth/register", h.registerOpen)
	// Email one-time codes (register / passwordless login)
	router.Post("/auth/email-code", h.requestEmailCode)
	router.Post("/auth/login/email-code", h.loginEmailCode)
	router.Post("/auth/register/email-code", h.registerEmailCode)
	// OAuth
	router.Get("/auth/oauth/providers", h.oauthProviders)
	router.Get("/auth/oauth/{provider}/start", h.oauthStart)
	router.Get("/auth/oauth/{provider}/callback", h.oauthCallback)
	router.Post("/auth/oauth/register/email-code", h.oauthRegisterEmailCode)
	router.Post("/auth/oauth/register/resend", h.oauthRegisterResend)
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
		// Account is preferred: email or username. Email is kept for backward compatibility.
		Account  string `json:"account"`
		Email    string `json:"email"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	identifier := strings.TrimSpace(request.Account)
	if identifier == "" {
		identifier = strings.TrimSpace(request.Email)
	}
	if identifier == "" {
		identifier = strings.TrimSpace(request.Username)
	}
	session, token, err := h.service.Login(r.Context(), identifier, request.Password, deviceSummary(r))
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

func (h *AuthHandler) registerOpen(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	session, token, err := h.service.RegisterOpen(r.Context(), auth.RegisterInput{
		Username: request.Username, Email: request.Email, Password: request.Password, Device: deviceSummary(r),
	})
	if err != nil {
		h.writeAuthError(w, r, err)
		return
	}
	h.setSessionCookie(w, token, session.ExpiresAt)
	WriteJSON(w, r, http.StatusCreated, authSessionData(session))
}

func (h *AuthHandler) requestEmailCode(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Email           string `json:"email"`
		Purpose         string `json:"purpose"` // register | login
		Username        string `json:"username"`
		Password        string `json:"password"`
		InvitationToken string `json:"invitationToken"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	purpose := auth.EmailCodePurpose(request.Purpose)
	payload := auth.RegisterPayload{
		Username: request.Username, Password: request.Password, InvitationToken: request.InvitationToken,
	}
	code, err := h.service.RequestEmailCode(r.Context(), request.Email, purpose, payload)
	if err != nil {
		h.writeAuthError(w, r, err)
		return
	}
	if code != "" && h.mailer != nil && h.mailer.MailConfigured(r.Context()) {
		msg := emailCodeMessage(h.instanceName, request.Email, code, request.Purpose)
		if sendErr := h.mailer.SendMail(r.Context(), msg); sendErr != nil {
			slog.Warn("send email code", "error", sendErr)
			WriteError(w, r, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "邮件发送失败，请检查 SMTP 配置", nil)
			return
		}
	} else if purpose == auth.EmailCodeRegister && code != "" {
		// Registration requires mail delivery.
		WriteError(w, r, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "服务器未配置邮件服务，无法发送验证码", nil)
		return
	}
	// Generic response for login (and success register after send).
	WriteJSON(w, r, http.StatusOK, map[string]any{
		"message": "若邮箱可用，验证码已发送，请在 10 分钟内完成验证。",
	})
}

func (h *AuthHandler) loginEmailCode(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	session, token, err := h.service.VerifyEmailCodeLogin(r.Context(), request.Email, request.Code, deviceSummary(r))
	if err != nil {
		h.writeAuthError(w, r, err)
		return
	}
	h.setSessionCookie(w, token, session.ExpiresAt)
	WriteJSON(w, r, http.StatusOK, authSessionData(session))
}

func (h *AuthHandler) registerEmailCode(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	session, token, err := h.service.VerifyEmailCodeRegister(r.Context(), request.Email, request.Code, deviceSummary(r))
	if err != nil {
		h.writeAuthError(w, r, err)
		return
	}
	h.setSessionCookie(w, token, session.ExpiresAt)
	WriteJSON(w, r, http.StatusCreated, authSessionData(session))
}

func (h *AuthHandler) oauthProviders(w http.ResponseWriter, r *http.Request) {
	providers, err := h.service.ListEnabledOAuth(r.Context())
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "读取 OAuth 配置失败", nil)
		return
	}
	list := make([]string, 0, len(providers))
	for _, p := range providers {
		list = append(list, string(p))
	}
	WriteJSON(w, r, http.StatusOK, map[string]any{"providers": list})
}

func (h *AuthHandler) oauthStart(w http.ResponseWriter, r *http.Request) {
	provider := auth.OAuthProvider(chi.URLParam(r, "provider"))
	invite := r.URL.Query().Get("invitationToken")
	authorizeURL, _, err := h.service.BeginOAuth(r.Context(), provider, invite)
	if err != nil {
		h.writeAuthError(w, r, err)
		return
	}
	http.Redirect(w, r, authorizeURL, http.StatusFound)
}

func (h *AuthHandler) oauthCallback(w http.ResponseWriter, r *http.Request) {
	provider := auth.OAuthProvider(chi.URLParam(r, "provider"))
	if r.URL.Query().Get("error") != "" {
		http.Redirect(w, r, "/login?oauth=denied", http.StatusFound)
		return
	}
	result, err := h.service.CompleteOAuth(
		r.Context(), provider, r.URL.Query().Get("code"), r.URL.Query().Get("state"), deviceSummary(r),
	)
	if err != nil {
		slog.Warn("oauth callback", "provider", provider, "error", err)
		http.Redirect(w, r, "/login?oauth="+oauthFailureReason(err), http.StatusFound)
		return
	}
	// First-time OAuth: email OTP (and invite when needed) before creating the account.
	if result.PendingEmail != "" {
		if result.PlainCode != "" && h.mailer != nil && h.mailer.MailConfigured(r.Context()) {
			msg := emailCodeMessage(h.instanceName, result.PendingEmail, result.PlainCode, "oauth_register")
			if sendErr := h.mailer.SendMail(r.Context(), msg); sendErr != nil {
				slog.Warn("send oauth register email code", "error", sendErr)
				http.Redirect(w, r, "/login?oauth=error", http.StatusFound)
				return
			}
		} else {
			// Registration via OAuth requires mail delivery for the OTP step.
			http.Redirect(w, r, "/login?oauth=mail_required", http.StatusFound)
			return
		}
		q := url.Values{}
		q.Set("email", result.PendingEmail)
		if result.NeedsInvite {
			q.Set("needsInvite", "1")
		}
		http.Redirect(w, r, "/oauth/complete?"+q.Encode(), http.StatusFound)
		return
	}
	h.setSessionCookie(w, result.PlainToken, result.Session.ExpiresAt)
	if result.Session.User.Role == "admin" {
		http.Redirect(w, r, "/admin", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/app?scope=personal", http.StatusFound)
}

func (h *AuthHandler) oauthRegisterEmailCode(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Email           string `json:"email"`
		Code            string `json:"code"`
		InvitationToken string `json:"invitationToken"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	session, token, err := h.service.VerifyOAuthRegister(
		r.Context(), request.Email, request.Code, request.InvitationToken, deviceSummary(r),
	)
	if err != nil {
		h.writeAuthError(w, r, err)
		return
	}
	h.setSessionCookie(w, token, session.ExpiresAt)
	WriteJSON(w, r, http.StatusCreated, authSessionData(session))
}

func (h *AuthHandler) oauthRegisterResend(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Email string `json:"email"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	code, needsInvite, err := h.service.ResendOAuthRegisterCode(r.Context(), request.Email)
	if err != nil {
		h.writeAuthError(w, r, err)
		return
	}
	if code != "" && h.mailer != nil && h.mailer.MailConfigured(r.Context()) {
		msg := emailCodeMessage(h.instanceName, request.Email, code, "oauth_register")
		if sendErr := h.mailer.SendMail(r.Context(), msg); sendErr != nil {
			WriteError(w, r, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "邮件发送失败，请检查 SMTP 配置", nil)
			return
		}
	} else {
		WriteError(w, r, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "服务器未配置邮件服务，无法发送验证码", nil)
		return
	}
	WriteJSON(w, r, http.StatusOK, map[string]any{
		"message":     "验证码已重新发送，请在 10 分钟内完成验证。",
		"needsInvite": needsInvite,
	})
}

// oauthFailureReason maps auth errors to login-page query values for clear UX.
func oauthFailureReason(err error) string {
	switch {
	case errors.Is(err, auth.ErrRegistrationClosed):
		// Closed mode, or invite still missing after OTP step.
		return "invite_required"
	case errors.Is(err, auth.ErrInvitationInvalid), errors.Is(err, auth.ErrInvitationExpired), errors.Is(err, auth.ErrInvitationExhausted):
		return "invite_required"
	case errors.Is(err, auth.ErrAccountDisabled):
		return "account_disabled"
	case errors.Is(err, auth.ErrMailRequired):
		return "mail_required"
	case errors.Is(err, auth.ErrOAuthDenied), errors.Is(err, auth.ErrOAuthState):
		return "denied"
	default:
		return "error"
	}
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
	case errors.Is(err, auth.ErrRegistrationClosed):
		WriteError(w, r, http.StatusForbidden, "REGISTRATION_CLOSED", "当前未开放公开注册", nil)
	case errors.Is(err, auth.ErrInvitationExpired):
		WriteError(w, r, http.StatusGone, "INVITATION_EXPIRED", "邀请已过期", nil)
	case errors.Is(err, auth.ErrInvitationExhausted):
		WriteError(w, r, http.StatusGone, "INVITATION_EXHAUSTED", "邀请使用次数已耗尽", nil)
	case errors.Is(err, auth.ErrInvitationInvalid):
		WriteError(w, r, http.StatusNotFound, "INVITATION_NOT_FOUND", "邀请不存在", nil)
	case errors.Is(err, auth.ErrEmailCodeInvalid), errors.Is(err, auth.ErrEmailCodeExpired):
		WriteError(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "验证码无效或已过期", nil)
	case errors.Is(err, auth.ErrMailRequired):
		WriteError(w, r, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "邮件服务未配置", nil)
	case errors.Is(err, auth.ErrOAuthNotConfigured):
		WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "该 OAuth 登录方式未启用", nil)
	case errors.Is(err, auth.ErrOAuthDenied), errors.Is(err, auth.ErrOAuthState):
		WriteError(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "OAuth 授权失败，请重试", nil)
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
