package httpapi

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/yixian-huang/navax/internal/auth"
	"github.com/yixian-huang/navax/internal/directoryadmin"
)

type DirectoryAdminHandler struct {
	auth    *auth.Service
	service *directoryadmin.Service
}

func NewDirectoryAdminHandler(authService *auth.Service, service *directoryadmin.Service) *DirectoryAdminHandler {
	return &DirectoryAdminHandler{auth: authService, service: service}
}

func (h *DirectoryAdminHandler) Mount(router chi.Router) {
	router.Route("/admin", func(management chi.Router) {
		management.Use(RequireSession(h.auth))
		management.Use(RequireAdmin)
		h.MountRoutes(management)
	})
}

func (h *DirectoryAdminHandler) MountRoutes(management chi.Router) {
	management.Get("/directory/categories", h.categories)
	management.Post("/directory/categories", h.createCategory)
	management.Patch("/directory/categories/{categoryId}", h.updateCategory)
	management.Delete("/directory/categories/{categoryId}", h.deleteCategory)
	management.Get("/directory/sites", h.sites)
	management.Post("/directory/sites", h.createSite)
	management.Patch("/directory/sites/{siteId}", h.updateSite)
	management.Delete("/directory/sites/{siteId}", h.deleteSite)
	management.Get("/links", h.links)
	management.Delete("/links/{siteId}", h.deleteLink)
}

func (h *DirectoryAdminHandler) categories(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.Categories(r.Context(), directoryActor(r))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	data := make([]map[string]any, 0, len(items))
	for _, item := range items {
		data = append(data, directoryCategoryData(item))
	}
	WriteJSON(w, r, http.StatusOK, data)
}

func (h *DirectoryAdminHandler) createCategory(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Name    string `json:"name"`
		Icon    string `json:"icon"`
		Enabled *bool  `json:"enabled"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	if request.Enabled == nil {
		WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "enabled 为必填项", nil)
		return
	}
	item, err := h.service.CreateCategory(r.Context(), directoryActor(r), directoryadmin.CategoryInput{
		Name: request.Name, Icon: request.Icon, Enabled: *request.Enabled,
	}, middleware.GetReqID(r.Context()))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusCreated, directoryCategoryData(item))
}

func (h *DirectoryAdminHandler) updateCategory(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Name    *string `json:"name"`
		Icon    *string `json:"icon"`
		Enabled *bool   `json:"enabled"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	if request.Name == nil || request.Icon == nil || request.Enabled == nil {
		WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "name、icon 和 enabled 均为必填项", nil)
		return
	}
	item, err := h.service.UpdateCategory(
		r.Context(), directoryActor(r), chi.URLParam(r, "categoryId"),
		directoryadmin.CategoryInput{Name: *request.Name, Icon: *request.Icon, Enabled: *request.Enabled},
		middleware.GetReqID(r.Context()),
	)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, directoryCategoryData(item))
}

func (h *DirectoryAdminHandler) deleteCategory(w http.ResponseWriter, r *http.Request) {
	if err := h.service.DeleteCategory(
		r.Context(), directoryActor(r), chi.URLParam(r, "categoryId"), middleware.GetReqID(r.Context()),
	); err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, nil)
}

func (h *DirectoryAdminHandler) sites(w http.ResponseWriter, r *http.Request) {
	page, pageSize, ok := readPagination(w, r)
	if !ok {
		return
	}
	result, err := h.service.Sites(r.Context(), directoryActor(r), directoryadmin.SiteFilter{
		Search: r.URL.Query().Get("search"), CategoryID: r.URL.Query().Get("categoryId"), Page: page, PageSize: pageSize,
	})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	items := make([]map[string]any, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, directorySiteData(item))
	}
	writePaginated(w, r, items, result.Page, result.PageSize, result.Total)
}

func (h *DirectoryAdminHandler) createSite(w http.ResponseWriter, r *http.Request) {
	var request struct {
		CategoryID  string `json:"categoryId"`
		Title       string `json:"title"`
		URL         string `json:"url"`
		Icon        string `json:"icon"`
		Description string `json:"description"`
		Enabled     *bool  `json:"enabled"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	if request.Enabled == nil {
		WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "enabled 为必填项", nil)
		return
	}
	item, err := h.service.CreateSite(r.Context(), directoryActor(r), directoryadmin.SiteInput{
		CategoryID: request.CategoryID, Title: request.Title, URL: request.URL,
		Icon: request.Icon, Description: request.Description, Enabled: *request.Enabled,
	}, middleware.GetReqID(r.Context()))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusCreated, directorySiteData(item))
}

func (h *DirectoryAdminHandler) updateSite(w http.ResponseWriter, r *http.Request) {
	var request struct {
		CategoryID  *string `json:"categoryId"`
		Title       *string `json:"title"`
		URL         *string `json:"url"`
		Icon        *string `json:"icon"`
		Description *string `json:"description"`
		Enabled     *bool   `json:"enabled"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	item, err := h.service.UpdateSite(
		r.Context(), directoryActor(r), chi.URLParam(r, "siteId"), directoryadmin.SitePatch{
			CategoryID: request.CategoryID, Title: request.Title, URL: request.URL,
			Icon: request.Icon, Description: request.Description, Enabled: request.Enabled,
		}, middleware.GetReqID(r.Context()),
	)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, directorySiteData(item))
}

func (h *DirectoryAdminHandler) deleteSite(w http.ResponseWriter, r *http.Request) {
	if err := h.service.DeleteSite(
		r.Context(), directoryActor(r), chi.URLParam(r, "siteId"), middleware.GetReqID(r.Context()),
	); err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, nil)
}

func (h *DirectoryAdminHandler) links(w http.ResponseWriter, r *http.Request) {
	page, pageSize, ok := readPagination(w, r)
	if !ok {
		return
	}
	result, err := h.service.Links(r.Context(), directoryActor(r), directoryadmin.LinkFilter{
		Search: r.URL.Query().Get("search"), OwnerID: r.URL.Query().Get("ownerId"), Page: page, PageSize: pageSize,
	})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	items := make([]map[string]any, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, adminLinkData(item))
	}
	writePaginated(w, r, items, result.Page, result.PageSize, result.Total)
}

func (h *DirectoryAdminHandler) deleteLink(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Reason string `json:"reason"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	if err := h.service.DeleteLink(
		r.Context(), directoryActor(r), chi.URLParam(r, "siteId"), request.Reason, middleware.GetReqID(r.Context()),
	); err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, nil)
}

func (h *DirectoryAdminHandler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, directoryadmin.ErrForbidden):
		WriteError(w, r, http.StatusForbidden, "ADMIN_REQUIRED", "需要管理员权限", nil)
	case errors.Is(err, directoryadmin.ErrInvalidInput):
		WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "目录请求参数无效", nil)
	case errors.Is(err, directoryadmin.ErrNotFound):
		WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "目录分类、站点或个人链接不存在", nil)
	case errors.Is(err, directoryadmin.ErrCategoryInUse):
		WriteError(w, r, http.StatusConflict, "CONFLICT", "请先删除或移动分类内的站点", nil)
	case errors.Is(err, directoryadmin.ErrConflict):
		WriteError(w, r, http.StatusConflict, "CONFLICT", "名称或网址已存在", nil)
	default:
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "目录管理操作失败", nil)
	}
}

func directoryActor(r *http.Request) directoryadmin.Actor {
	session, _ := SessionFromContext(r.Context())
	return directoryadmin.Actor{
		ID: session.User.ID, Username: session.User.Username,
		Role: session.User.Role, Status: session.User.Status,
	}
}

func directoryCategoryData(item directoryadmin.Category) map[string]any {
	return map[string]any{
		"id": item.ID, "name": item.Name, "icon": item.Icon,
		"sortOrder": item.SortOrder, "enabled": item.Enabled, "siteCount": item.SiteCount,
	}
}

func directorySiteData(item directoryadmin.Site) map[string]any {
	return map[string]any{
		"id": item.ID, "categoryId": item.CategoryID, "categoryName": item.CategoryName,
		"title": item.Title, "url": item.URL, "icon": item.Icon, "description": item.Description,
		"sortOrder": item.SortOrder, "enabled": item.Enabled,
	}
}

func adminLinkData(item directoryadmin.AdminLink) map[string]any {
	return map[string]any{
		"id": item.ID, "categoryId": item.CategoryID, "categoryName": item.CategoryName,
		"ownerId": item.OwnerID, "ownerName": item.OwnerName,
		"title": item.Title, "url": item.URL, "icon": item.Icon,
		"description": item.Description, "sortOrder": item.SortOrder, "enabled": item.Enabled,
		"createdAt": item.CreatedAt, "updatedAt": item.UpdatedAt,
	}
}
