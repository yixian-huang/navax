package httpapi

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/yixian-huang/navax/internal/catalog"
)

type CatalogHandler struct{ service *catalog.Service }

func NewCatalogHandler(service *catalog.Service) *CatalogHandler {
	return &CatalogHandler{service: service}
}

func (h *CatalogHandler) Mount(router chi.Router) {
	router.Get("/public/config", h.config)
	router.Get("/themes", h.themes)
	router.Get("/public/directory", h.directory)
	router.Get("/public/discover", h.discover)
}

func (h *CatalogHandler) config(w http.ResponseWriter, r *http.Request) {
	config, err := h.service.Config(r.Context())
	if err != nil {
		WriteError(w, r, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "读取公开配置失败", nil)
		return
	}
	WriteJSON(w, r, http.StatusOK, config)
}

func (h *CatalogHandler) themes(w http.ResponseWriter, r *http.Request) {
	themes, err := h.service.Themes(r.Context())
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "读取主题失败", nil)
		return
	}
	data := make([]map[string]any, 0, len(themes))
	for _, item := range themes {
		data = append(data, themeData(item))
	}
	WriteJSON(w, r, http.StatusOK, data)
}

func (h *CatalogHandler) directory(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.Directory(
		r.Context(), r.URL.Query().Get("search"), r.URL.Query().Get("categoryId"),
		queryInt(r, "page", 1), queryInt(r, "pageSize", 20),
	)
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "读取推荐目录失败", nil)
		return
	}
	writePaginated(w, r, result.Items, result.Page, result.PageSize, result.Total)
}

func (h *CatalogHandler) discover(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.Discover(
		r.Context(), r.URL.Query().Get("search"), r.URL.Query().Get("tag"), r.URL.Query().Get("sort"),
		queryInt(r, "page", 1), queryInt(r, "pageSize", 20),
	)
	if err != nil {
		if errors.Is(err, catalog.ErrDiscoverDisabled) {
			WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "发现页已关闭", nil)
			return
		}
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "读取发现页失败", nil)
		return
	}
	writePaginated(w, r, result.Items, result.Page, result.PageSize, result.Total)
}
