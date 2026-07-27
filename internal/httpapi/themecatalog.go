package httpapi

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/yixian-huang/navax/internal/auth"
	"github.com/yixian-huang/navax/internal/themecatalog"
)

// ThemeCatalogHandler exposes the private→catalog promotion request/approve
// workflow. Owner-facing routes live under /me, admin routes under /admin.
type ThemeCatalogHandler struct {
	auth    *auth.Service
	service *themecatalog.Service
}

func NewThemeCatalogHandler(authService *auth.Service, service *themecatalog.Service) *ThemeCatalogHandler {
	return &ThemeCatalogHandler{auth: authService, service: service}
}

func (h *ThemeCatalogHandler) MountUserRoutes(router chi.Router) {
	router.Post("/me/themes/{themeId}/catalog-request", h.request)
	router.Delete("/me/themes/{themeId}/catalog-request", h.cancel)
}

func (h *ThemeCatalogHandler) MountAdminRoutes(router chi.Router) {
	router.Get("/theme-catalog-requests", h.list)
	router.Patch("/theme-catalog-requests/{requestId}", h.review)
}

func (h *ThemeCatalogHandler) request(w http.ResponseWriter, r *http.Request) {
	item, err := h.service.Request(
		r.Context(), themeCatalogActor(r), chi.URLParam(r, "themeId"), middleware.GetReqID(r.Context()),
	)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusCreated, themeCatalogRequestData(item))
}

func (h *ThemeCatalogHandler) cancel(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Cancel(
		r.Context(), themeCatalogActor(r), chi.URLParam(r, "themeId"), middleware.GetReqID(r.Context()),
	); err != nil {
		h.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ThemeCatalogHandler) list(w http.ResponseWriter, r *http.Request) {
	page, pageSize, ok := readPagination(w, r)
	if !ok {
		return
	}
	result, err := h.service.Requests(r.Context(), themeCatalogActor(r), r.URL.Query().Get("status"), page, pageSize)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	items := make([]map[string]any, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, themeCatalogRequestData(item))
	}
	writePaginated(w, r, items, result.Page, result.PageSize, result.Total)
}

func (h *ThemeCatalogHandler) review(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	item, err := h.service.Review(
		r.Context(), themeCatalogActor(r), chi.URLParam(r, "requestId"),
		request.Decision, request.Reason, middleware.GetReqID(r.Context()),
	)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, themeCatalogRequestData(item))
}

func (h *ThemeCatalogHandler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, themecatalog.ErrForbidden):
		WriteError(w, r, http.StatusForbidden, "ADMIN_REQUIRED", "需要管理员权限", nil)
	case errors.Is(err, themecatalog.ErrSlugConflict):
		WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "该主题 slug 已在官方目录中被占用", nil)
	case errors.Is(err, themecatalog.ErrThemeNotEligible):
		WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "主题必须已启用且有可用版本才能提交审核", nil)
	case errors.Is(err, themecatalog.ErrInvalidInput):
		WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "请求参数无效", nil)
	case errors.Is(err, themecatalog.ErrConflict):
		WriteError(w, r, http.StatusConflict, "CONFLICT", "该主题已有生效的目录申请", nil)
	case errors.Is(err, themecatalog.ErrInvalidTransition):
		WriteError(w, r, http.StatusConflict, "CONFLICT", "当前申请状态不允许此操作", nil)
	case errors.Is(err, themecatalog.ErrNotFound):
		WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "目录申请不存在", nil)
	default:
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "目录审核操作失败", nil)
	}
}

func themeCatalogActor(r *http.Request) themecatalog.Actor {
	session, _ := SessionFromContext(r.Context())
	return themecatalog.Actor{
		ID: session.User.ID, Username: session.User.Username,
		Role: session.User.Role, Status: session.User.Status,
	}
}

func themeCatalogRequestData(item themecatalog.Request) map[string]any {
	return map[string]any{
		"id": item.ID, "themeId": item.ThemeID, "themeName": item.ThemeName, "slug": item.Slug,
		"ownerId": item.OwnerID, "ownerName": item.OwnerName, "status": item.Status,
		"reason": item.Reason, "appliedAt": item.AppliedAt, "reviewedAt": item.ReviewedAt,
	}
}
