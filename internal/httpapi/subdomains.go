package httpapi

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/yixian-huang/navax/internal/auth"
	"github.com/yixian-huang/navax/internal/subdomains"
)

type SubdomainHandler struct {
	auth    *auth.Service
	service *subdomains.Service
}

func NewSubdomainHandler(authService *auth.Service, service *subdomains.Service) *SubdomainHandler {
	return &SubdomainHandler{auth: authService, service: service}
}

func (h *SubdomainHandler) Mount(router chi.Router) {
	router.Group(func(protected chi.Router) {
		protected.Use(RequireSession(h.auth))
		h.MountUserRoutes(protected)
		protected.Route("/admin", func(management chi.Router) {
			management.Use(RequireAdmin)
			h.MountAdminRoutes(management)
		})
	})
}

func (h *SubdomainHandler) MountUserRoutes(router chi.Router) {
	router.Get("/me/subdomain", h.mine)
	router.Post("/me/subdomain", h.apply)
	router.Patch("/me/subdomain", h.setCustomDomain)
	router.Delete("/me/subdomain", h.cancel)
}

func (h *SubdomainHandler) MountAdminRoutes(router chi.Router) {
	router.Get("/subdomains", h.requests)
	router.Patch("/subdomains/{requestId}", h.review)
}

func (h *SubdomainHandler) mine(w http.ResponseWriter, r *http.Request) {
	session, _ := SessionFromContext(r.Context())
	item, err := h.service.Mine(r.Context(), session.User.ID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	if item == nil {
		WriteJSON(w, r, http.StatusOK, nil)
		return
	}
	WriteJSON(w, r, http.StatusOK, subdomainData(*item))
}

func (h *SubdomainHandler) apply(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Label string `json:"label"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	session, _ := SessionFromContext(r.Context())
	item, err := h.service.Apply(
		r.Context(), session.User.ID, session.User.Username, request.Label, middleware.GetReqID(r.Context()),
	)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusCreated, subdomainData(item))
}

func (h *SubdomainHandler) cancel(w http.ResponseWriter, r *http.Request) {
	session, _ := SessionFromContext(r.Context())
	if err := h.service.Cancel(r.Context(), session.User.ID, session.User.Username, middleware.GetReqID(r.Context())); err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, nil)
}

func (h *SubdomainHandler) setCustomDomain(w http.ResponseWriter, r *http.Request) {
	var request struct {
		CustomDomain *string `json:"customDomain"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	domain := ""
	if request.CustomDomain != nil {
		domain = *request.CustomDomain
	}
	session, _ := SessionFromContext(r.Context())
	item, err := h.service.SetCustomDomain(
		r.Context(), session.User.ID, session.User.Username, domain, middleware.GetReqID(r.Context()),
	)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, subdomainData(item))
}

func (h *SubdomainHandler) requests(w http.ResponseWriter, r *http.Request) {
	page, pageSize, ok := readPagination(w, r)
	if !ok {
		return
	}
	result, err := h.service.Requests(
		r.Context(), subdomainActor(r), r.URL.Query().Get("status"), page, pageSize,
	)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	items := make([]map[string]any, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, subdomainData(item))
	}
	writePaginated(w, r, items, result.Page, result.PageSize, result.Total)
}

func (h *SubdomainHandler) review(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	item, err := h.service.Review(
		r.Context(), subdomainActor(r), chi.URLParam(r, "requestId"),
		request.Decision, request.Reason, middleware.GetReqID(r.Context()),
	)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, subdomainData(item))
}

func (h *SubdomainHandler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, subdomains.ErrForbidden):
		WriteError(w, r, http.StatusForbidden, "ADMIN_REQUIRED", "需要管理员权限", nil)
	case errors.Is(err, subdomains.ErrInvalidLabel):
		WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "子域名仅支持 1 至 30 位小写字母、数字和连字符，且不能以连字符开头或结尾", nil)
	case errors.Is(err, subdomains.ErrReservedLabel):
		WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "该子域名为系统保留名称", nil)
	case errors.Is(err, subdomains.ErrInvalidInput):
		WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "子域名请求参数无效", nil)
	case errors.Is(err, subdomains.ErrUnavailable):
		WriteError(w, r, http.StatusConflict, "CONFLICT", "当前实例尚未启用子域名", nil)
	case errors.Is(err, subdomains.ErrConflict):
		WriteError(w, r, http.StatusConflict, "CONFLICT", "该用户已有生效申请，或子域名已被占用", nil)
	case errors.Is(err, subdomains.ErrInvalidTransition):
		WriteError(w, r, http.StatusConflict, "CONFLICT", "当前申请状态不允许此操作", nil)
	case errors.Is(err, subdomains.ErrNotFound):
		WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "子域名申请不存在", nil)
	default:
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "子域名操作失败", nil)
	}
}

func subdomainActor(r *http.Request) subdomains.Actor {
	session, _ := SessionFromContext(r.Context())
	return subdomains.Actor{
		ID: session.User.ID, Username: session.User.Username,
		Role: session.User.Role, Status: session.User.Status,
	}
}

func subdomainData(item subdomains.Request) map[string]any {
	data := map[string]any{
		"id": item.ID, "userId": item.UserID, "label": item.Label,
		"fullDomain": item.FullDomain, "customDomain": item.CustomDomain,
		"status": item.Status, "appliedAt": item.AppliedAt,
		"reviewedAt": item.ReviewedAt, "reason": item.Reason,
	}
	if item.Username != "" {
		data["username"] = item.Username
	}
	return data
}
