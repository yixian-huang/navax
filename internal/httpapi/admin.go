package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	adminpkg "github.com/yixian-huang/navax/internal/admin"
	"github.com/yixian-huang/navax/internal/auth"
)

type AdminHandlerOptions struct {
	Version      string
	StartedAt    time.Time
	InstanceName string
	Mailer       Mailer
}

type AdminHandler struct {
	auth         *auth.Service
	service      *adminpkg.Service
	version      string
	startedAt    time.Time
	instanceName string
	mailer       Mailer
}

func NewAdminHandler(authService *auth.Service, service *adminpkg.Service, options ...AdminHandlerOptions) *AdminHandler {
	option := AdminHandlerOptions{Version: "dev", StartedAt: time.Now()}
	if len(options) > 0 {
		if options[0].Version != "" {
			option.Version = options[0].Version
		}
		if !options[0].StartedAt.IsZero() {
			option.StartedAt = options[0].StartedAt
		}
		option.InstanceName = options[0].InstanceName
		option.Mailer = options[0].Mailer
	}
	return &AdminHandler{
		auth: authService, service: service, version: option.Version, startedAt: option.StartedAt,
		instanceName: option.InstanceName, mailer: option.Mailer,
	}
}

func (h *AdminHandler) Mount(router chi.Router) {
	router.Route("/admin", func(management chi.Router) {
		management.Use(RequireSession(h.auth))
		management.Use(RequireAdmin)
		h.MountRoutes(management)
	})
}

// MountRoutes mounts relative admin routes when authentication and role
// middleware are already installed by the application composition root.
func (h *AdminHandler) MountRoutes(management chi.Router) {
	management.Get("/overview", h.overview)
	management.Get("/users", h.users)
	management.Get("/users/{userId}", h.user)
	management.Patch("/users/{userId}", h.updateUserStatus)
	management.Delete("/users/{userId}/sessions", h.revokeUserSessions)
	management.Post("/users/{userId}/password-reset", h.resetUserPassword)
	management.Get("/invitations", h.invitations)
	management.Post("/invitations", h.createInvitation)
	management.Delete("/invitations/{invitationId}", h.revokeInvitation)
	management.Get("/themes", h.themes)
	management.Patch("/themes/{themeId}", h.updateTheme)
	management.Get("/settings", h.settings)
	management.Patch("/settings", h.updateSettings)
	management.Get("/audit", h.audit)
}

func (h *AdminHandler) overview(w http.ResponseWriter, r *http.Request) {
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	uptime := time.Since(h.startedAt)
	if uptime < 0 {
		uptime = 0
	}
	data, err := h.service.Overview(r.Context(), actorFromRequest(r), adminpkg.Health{
		Status: "healthy", UptimeSeconds: int64(uptime.Seconds()),
		Version: h.version, GoVersion: runtime.Version(), MemoryBytes: memory.Alloc,
	})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	recent := make([]map[string]any, 0, len(data.RecentActions))
	for _, item := range data.RecentActions {
		recent = append(recent, auditData(item))
	}
	WriteJSON(w, r, http.StatusOK, map[string]any{
		"totalUsers": data.TotalUsers, "activeUsers": data.ActiveUsers,
		"activeInvitations": data.ActiveInvitations, "publicPages": data.PublicPages,
		"health": map[string]any{
			"status": data.Health.Status, "uptimeSeconds": data.Health.UptimeSeconds,
			"version": data.Health.Version, "goVersion": data.Health.GoVersion,
			"memoryBytes": data.Health.MemoryBytes,
		},
		"recentActions": recent,
	})
}

func (h *AdminHandler) users(w http.ResponseWriter, r *http.Request) {
	page, pageSize, ok := readPagination(w, r)
	if !ok {
		return
	}
	result, err := h.service.Users(r.Context(), actorFromRequest(r), adminpkg.UserFilter{
		Search: r.URL.Query().Get("search"), Status: r.URL.Query().Get("status"), Page: page, PageSize: pageSize,
	})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	items := make([]map[string]any, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, userData(item))
	}
	writePaginated(w, r, items, result.Page, result.PageSize, result.Total)
}

func (h *AdminHandler) user(w http.ResponseWriter, r *http.Request) {
	item, err := h.service.User(r.Context(), actorFromRequest(r), chi.URLParam(r, "userId"))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, userData(item))
}

func (h *AdminHandler) updateUserStatus(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	item, err := h.service.SetUserStatus(
		r.Context(), actorFromRequest(r), chi.URLParam(r, "userId"),
		request.Status, request.Reason, middleware.GetReqID(r.Context()),
	)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, userData(item))
}

func (h *AdminHandler) revokeUserSessions(w http.ResponseWriter, r *http.Request) {
	err := h.service.RevokeUserSessions(
		r.Context(), actorFromRequest(r), chi.URLParam(r, "userId"), middleware.GetReqID(r.Context()),
	)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, nil)
}

func (h *AdminHandler) invitations(w http.ResponseWriter, r *http.Request) {
	page, pageSize, ok := readPagination(w, r)
	if !ok {
		return
	}
	result, err := h.service.Invitations(r.Context(), actorFromRequest(r), page, pageSize)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	items := make([]map[string]any, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, invitationData(item))
	}
	writePaginated(w, r, items, result.Page, result.PageSize, result.Total)
}

func (h *AdminHandler) createInvitation(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Email         string `json:"email"`
		MaxUses       int    `json:"maxUses"`
		ExpiresInDays int    `json:"expiresInDays"`
		SendEmail     bool   `json:"sendEmail"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	actor := actorFromRequest(r)
	settings, err := h.service.Settings(r.Context(), actor)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	created, err := h.service.CreateInvitation(r.Context(), actor, adminpkg.InvitationCreate{
		Email: request.Email, MaxUses: request.MaxUses, ExpiresInDays: request.ExpiresInDays,
		SendEmail: request.SendEmail, PublicBaseURL: settings.PublicBaseURL,
		RequestID: middleware.GetReqID(r.Context()),
	})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	data := invitationData(created.Invitation)
	data["token"] = created.Token
	data["inviteUrl"] = created.InviteURL
	data["emailSent"] = h.maybeSendInvite(r.Context(), request.SendEmail, created)
	WriteJSON(w, r, http.StatusCreated, data)
}

// maybeSendInvite delivers the invitation email when the admin asked for it and
// an SMTP provider is configured. It is best-effort: the invitation already
// exists and its URL is returned regardless, so a delivery failure is logged
// rather than surfaced as a request error.
func (h *AdminHandler) maybeSendInvite(ctx context.Context, requested bool, created adminpkg.InvitationCreated) bool {
	if !requested || created.Email == nil || h.mailer == nil || !h.mailer.MailConfigured(ctx) {
		return false
	}
	message := inviteMessage(h.instanceName, *created.Email, created.InviteURL, created.ExpiresAt)
	if err := h.mailer.SendMail(ctx, message); err != nil {
		slog.Warn("send invitation email", "error", err, "invitation", created.ID)
		return false
	}
	return true
}

// resetUserPassword issues a single-use reset link for a user. It works without
// SMTP: the link is always returned to the administrator (to share out-of-band)
// and additionally emailed to the user when a provider is configured.
func (h *AdminHandler) resetUserPassword(w http.ResponseWriter, r *http.Request) {
	actor := actorFromRequest(r)
	userID := chi.URLParam(r, "userId")
	// Authorize the admin and confirm the target user exists (404 otherwise).
	if _, err := h.service.User(r.Context(), actor, userID); err != nil {
		h.writeError(w, r, err)
		return
	}
	settings, err := h.service.Settings(r.Context(), actor)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	result, err := h.auth.IssuePasswordReset(r.Context(), userID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	resetURL := strings.TrimRight(settings.PublicBaseURL, "/") + "/reset-password?token=" + url.QueryEscape(result.Token)
	emailSent := false
	if h.mailer != nil && h.mailer.MailConfigured(r.Context()) {
		message := passwordResetMessage(h.instanceName, result.User.Email, resetURL, result.ExpiresAt)
		if sendErr := h.mailer.SendMail(r.Context(), message); sendErr != nil {
			slog.Warn("send admin password reset email", "error", sendErr, "user", userID)
		} else {
			emailSent = true
		}
	}
	if auditErr := h.service.WriteAudit(r.Context(), actor, "user.password.reset", "user", userID,
		map[string]any{"emailSent": emailSent}, middleware.GetReqID(r.Context())); auditErr != nil {
		slog.Warn("write password reset audit", "error", auditErr, "user", userID)
	}
	WriteJSON(w, r, http.StatusOK, map[string]any{
		"resetUrl": resetURL, "expiresAt": result.ExpiresAt, "emailSent": emailSent,
	})
}

func (h *AdminHandler) revokeInvitation(w http.ResponseWriter, r *http.Request) {
	item, err := h.service.RevokeInvitation(
		r.Context(), actorFromRequest(r), chi.URLParam(r, "invitationId"), middleware.GetReqID(r.Context()),
	)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, invitationData(item))
}

func (h *AdminHandler) themes(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.Themes(r.Context(), actorFromRequest(r))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	data := make([]map[string]any, 0, len(items))
	for _, item := range items {
		data = append(data, themeData(item))
	}
	WriteJSON(w, r, http.StatusOK, data)
}

func (h *AdminHandler) updateTheme(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Enabled *bool `json:"enabled"`
		Default *bool `json:"default"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	item, err := h.service.UpdateTheme(r.Context(), actorFromRequest(r), chi.URLParam(r, "themeId"), adminpkg.ThemePatch{
		Enabled: request.Enabled, Default: request.Default, RequestID: middleware.GetReqID(r.Context()),
	})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, themeData(item))
}

func (h *AdminHandler) settings(w http.ResponseWriter, r *http.Request) {
	data, err := h.service.Settings(r.Context(), actorFromRequest(r))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, settingsData(data))
}

func (h *AdminHandler) updateSettings(w http.ResponseWriter, r *http.Request) {
	var request struct {
		InstanceName     *string `json:"instanceName"`
		PublicBaseURL    *string `json:"publicBaseUrl"`
		RegistrationMode *string `json:"registrationMode"`
		Limits           *struct {
			MaxCategoriesPerPage *int   `json:"maxCategoriesPerPage"`
			MaxSitesPerPage      *int   `json:"maxSitesPerPage"`
			MaxUploadBytes       *int64 `json:"maxUploadBytes"`
		} `json:"limits"`
		Analytics *struct {
			Enabled       *bool `json:"enabled"`
			RetentionDays *int  `json:"retentionDays"`
		} `json:"analytics"`
		Domain *struct {
			RootDomain        json.RawMessage `json:"rootDomain"`
			SubdomainsEnabled *bool           `json:"subdomainsEnabled"`
		} `json:"domain"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	patch := adminpkg.SystemSettingsPatch{
		InstanceName: request.InstanceName, PublicBaseURL: request.PublicBaseURL,
		RegistrationMode: request.RegistrationMode, RequestID: middleware.GetReqID(r.Context()),
	}
	if request.Limits != nil {
		patch.Limits = &adminpkg.LimitsPatch{
			MaxCategoriesPerPage: request.Limits.MaxCategoriesPerPage,
			MaxSitesPerPage:      request.Limits.MaxSitesPerPage,
			MaxUploadBytes:       request.Limits.MaxUploadBytes,
		}
	}
	if request.Analytics != nil {
		patch.Analytics = &adminpkg.AnalyticsPatch{Enabled: request.Analytics.Enabled, RetentionDays: request.Analytics.RetentionDays}
	}
	if request.Domain != nil {
		patch.Domain = &adminpkg.DomainPatch{SubdomainsEnabled: request.Domain.SubdomainsEnabled}
		if request.Domain.RootDomain != nil {
			var value *string
			if err := json.Unmarshal(request.Domain.RootDomain, &value); err != nil {
				WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "根域名格式无效", nil)
				return
			}
			patch.Domain.RootDomain = &value
		}
	}
	data, err := h.service.UpdateSettings(r.Context(), actorFromRequest(r), patch)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, settingsData(data))
}

func (h *AdminHandler) audit(w http.ResponseWriter, r *http.Request) {
	page, pageSize, ok := readPagination(w, r)
	if !ok {
		return
	}
	result, err := h.service.Audit(r.Context(), actorFromRequest(r), adminpkg.AuditFilter{
		Action: r.URL.Query().Get("action"), Page: page, PageSize: pageSize,
	})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	items := make([]map[string]any, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, auditData(item))
	}
	writePaginated(w, r, items, result.Page, result.PageSize, result.Total)
}

func (h *AdminHandler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, adminpkg.ErrForbidden):
		WriteError(w, r, http.StatusForbidden, "ADMIN_REQUIRED", "需要管理员权限", nil)
	case errors.Is(err, adminpkg.ErrInvalidInput):
		WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "管理请求参数无效", nil)
	case errors.Is(err, adminpkg.ErrNotFound):
		WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "资源不存在", nil)
	case errors.Is(err, adminpkg.ErrSelfDisable):
		WriteError(w, r, http.StatusConflict, "CONFLICT", "不能停用当前管理员账号", nil)
	case errors.Is(err, adminpkg.ErrDefaultTheme):
		WriteError(w, r, http.StatusConflict, "CONFLICT", "默认主题必须保持启用", nil)
	case errors.Is(err, adminpkg.ErrInvitationState):
		WriteError(w, r, http.StatusConflict, "CONFLICT", "邀请已被撤销", nil)
	case errors.Is(err, adminpkg.ErrConflict):
		WriteError(w, r, http.StatusConflict, "CONFLICT", "操作与当前状态冲突", nil)
	default:
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "管理操作失败", nil)
	}
}

func actorFromRequest(r *http.Request) adminpkg.Actor {
	session, _ := SessionFromContext(r.Context())
	return adminpkg.Actor{
		ID: session.User.ID, Username: session.User.Username,
		Role: session.User.Role, Status: session.User.Status,
	}
}

func readPagination(w http.ResponseWriter, r *http.Request) (int, int, bool) {
	page, ok := positiveQueryInteger(w, r, "page", 1, 0)
	if !ok {
		return 0, 0, false
	}
	pageSize, ok := positiveQueryInteger(w, r, "pageSize", 20, 100)
	return page, pageSize, ok
}

func positiveQueryInteger(w http.ResponseWriter, r *http.Request, name string, fallback, maximum int) (int, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return fallback, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 || maximum > 0 && value > maximum {
		WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", name+" 参数无效", nil)
		return 0, false
	}
	return value, true
}

func writePaginated(w http.ResponseWriter, r *http.Request, data any, page, pageSize, total int) {
	totalPages := 0
	if pageSize > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}
	WriteRawJSON(w, r, http.StatusOK, map[string]any{
		"code": "OK", "data": data,
		"meta": map[string]any{
			"requestId": middleware.GetReqID(r.Context()), "page": page, "pageSize": pageSize,
			"total": total, "totalPages": totalPages,
		},
	})
}

func invitationData(item adminpkg.Invitation) map[string]any {
	return map[string]any{
		"id": item.ID, "tokenPreview": item.TokenPreview, "creatorName": item.CreatorName,
		"email": item.Email, "maxUses": item.MaxUses, "usedCount": item.UsedCount,
		"expiresAt": item.ExpiresAt, "revokedAt": item.RevokedAt, "createdAt": item.CreatedAt,
	}
}

func themeData(item adminpkg.Theme) map[string]any {
	return map[string]any{
		"id": item.ID, "name": item.Name, "version": item.Version, "author": item.Author,
		"description": item.Description, "mode": item.Mode, "preview": item.Preview,
		"enabled": item.Enabled, "default": item.Default,
	}
}

func settingsData(item adminpkg.SystemSettings) map[string]any {
	return map[string]any{
		"instanceName": item.InstanceName, "publicBaseUrl": item.PublicBaseURL,
		"registrationMode": item.RegistrationMode,
		"limits": map[string]any{
			"maxCategoriesPerPage": item.Limits.MaxCategoriesPerPage,
			"maxSitesPerPage":      item.Limits.MaxSitesPerPage, "maxUploadBytes": item.Limits.MaxUploadBytes,
		},
		"analytics": map[string]any{"enabled": item.Analytics.Enabled, "retentionDays": item.Analytics.RetentionDays},
		"domain":    map[string]any{"rootDomain": item.Domain.RootDomain, "subdomainsEnabled": item.Domain.SubdomainsEnabled},
	}
}

func auditData(item adminpkg.AuditEntry) map[string]any {
	return map[string]any{
		"id": item.ID, "actorId": item.ActorID, "actorName": item.ActorName,
		"action": item.Action, "targetType": item.TargetType, "targetId": item.TargetID,
		"detail": item.Detail, "createdAt": item.CreatedAt,
	}
}
