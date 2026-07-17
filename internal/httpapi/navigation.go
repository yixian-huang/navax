package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/yixian-huang/navax/internal/idempotency"
	"github.com/yixian-huang/navax/internal/navigation"
)

type NavigationHandler struct {
	service       *navigation.Service
	publicBaseURL string
	idempotency   *idempotency.Service
}

type NavigationHandlerOptions struct {
	Idempotency *idempotency.Service
}

func NewNavigationHandler(service *navigation.Service, publicBaseURL string, options ...NavigationHandlerOptions) *NavigationHandler {
	handler := &NavigationHandler{service: service, publicBaseURL: publicBaseURL}
	if len(options) > 0 {
		handler.idempotency = options[0].Idempotency
	}
	return handler
}

func (h *NavigationHandler) MountPublic(router chi.Router) {
	router.Get("/public/home", h.publicHome)
	router.Get("/public/pages/{slug}", h.publicPage)
}

func (h *NavigationHandler) MountProtected(router chi.Router) {
	router.Get("/pages/current", h.currentPage)
	router.Get("/pages/{pageId}", h.page)
	router.Patch("/pages/{pageId}", h.updatePage)
	router.Get("/pages/{pageId}/categories", h.categories)
	router.Post("/pages/{pageId}/categories", h.createCategory)
	router.Patch("/pages/{pageId}/categories/{categoryId}", h.updateCategory)
	router.Delete("/pages/{pageId}/categories/{categoryId}", h.deleteCategory)
	router.Get("/pages/{pageId}/sites", h.sites)
	router.Post("/pages/{pageId}/sites", h.createSite)
	router.Patch("/pages/{pageId}/sites/{siteId}", h.updateSite)
	router.Delete("/pages/{pageId}/sites/{siteId}", h.deleteSite)
	router.Put("/pages/{pageId}/content-order", h.replaceContentOrder)
	router.Get("/pages/{pageId}/settings", h.settings)
	router.Put("/pages/{pageId}/settings", h.replaceSettings)
	router.Get("/pages/{pageId}/preview", h.preview)
	router.Get("/pages/{pageId}/publication", h.publication)
	router.Put("/pages/{pageId}/publication", h.replacePublication)
	router.Delete("/pages/{pageId}/publication", h.unpublish)
	router.Post("/pages/{pageId}/publish", h.publish)
}

func (h *NavigationHandler) currentPage(w http.ResponseWriter, r *http.Request) {
	var kind navigation.PageKind
	switch r.URL.Query().Get("scope") {
	case "personal":
		kind = navigation.PageKindPersonal
	case "system":
		kind = navigation.PageKindSystem
	default:
		WriteError(w, r, http.StatusBadRequest, "VALIDATION_FAILED", "scope 必须是 personal 或 system", nil)
		return
	}
	page, err := h.service.CurrentPage(r.Context(), navigationActor(r), kind)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, page)
}

func (h *NavigationHandler) page(w http.ResponseWriter, r *http.Request) {
	page, err := h.service.PageDraft(r.Context(), navigationActor(r), chi.URLParam(r, "pageId"))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, page)
}

func (h *NavigationHandler) updatePage(w http.ResponseWriter, r *http.Request) {
	var request struct {
		ExpectedRevision int     `json:"expectedRevision"`
		Title            *string `json:"title"`
		Description      *string `json:"description"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	page, err := h.service.UpdatePage(r.Context(), navigationActor(r), chi.URLParam(r, "pageId"), navigation.PagePatch{
		ExpectedRevision: request.ExpectedRevision, Title: request.Title, Description: request.Description,
	})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, page)
}

func (h *NavigationHandler) categories(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.Categories(r.Context(), navigationActor(r), chi.URLParam(r, "pageId"))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, items)
}

func (h *NavigationHandler) createCategory(w http.ResponseWriter, r *http.Request) {
	var request navigation.CategoryInput
	if !decodeJSON(w, r, &request) {
		return
	}
	category, err := h.service.CreateCategory(r.Context(), navigationActor(r), chi.URLParam(r, "pageId"), request)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusCreated, category)
}

func (h *NavigationHandler) updateCategory(w http.ResponseWriter, r *http.Request) {
	var request navigation.CategoryPatch
	if !decodeJSON(w, r, &request) {
		return
	}
	category, err := h.service.UpdateCategory(
		r.Context(), navigationActor(r), chi.URLParam(r, "pageId"), chi.URLParam(r, "categoryId"), request,
	)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, category)
}

func (h *NavigationHandler) deleteCategory(w http.ResponseWriter, r *http.Request) {
	err := h.service.DeleteCategory(
		r.Context(), navigationActor(r), chi.URLParam(r, "pageId"), chi.URLParam(r, "categoryId"),
		navigation.DeleteCategoryMode(r.URL.Query().Get("mode")),
	)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, nil)
}

func (h *NavigationHandler) sites(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.Sites(
		r.Context(), navigationActor(r), chi.URLParam(r, "pageId"),
		r.URL.Query().Get("categoryId"), r.URL.Query().Get("search"),
	)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, items)
}

func (h *NavigationHandler) createSite(w http.ResponseWriter, r *http.Request) {
	var request navigation.SiteInput
	if !decodeJSON(w, r, &request) {
		return
	}
	site, err := h.service.CreateSite(r.Context(), navigationActor(r), chi.URLParam(r, "pageId"), request)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusCreated, site)
}

func (h *NavigationHandler) updateSite(w http.ResponseWriter, r *http.Request) {
	var request navigation.SitePatch
	if !decodeJSON(w, r, &request) {
		return
	}
	site, err := h.service.UpdateSite(
		r.Context(), navigationActor(r), chi.URLParam(r, "pageId"), chi.URLParam(r, "siteId"), request,
	)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, site)
}

func (h *NavigationHandler) deleteSite(w http.ResponseWriter, r *http.Request) {
	if err := h.service.DeleteSite(r.Context(), navigationActor(r), chi.URLParam(r, "pageId"), chi.URLParam(r, "siteId")); err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, nil)
}

func (h *NavigationHandler) replaceContentOrder(w http.ResponseWriter, r *http.Request) {
	var request struct {
		ExpectedRevision int `json:"expectedRevision"`
		Categories       []struct {
			ID      string   `json:"id"`
			SiteIDs []string `json:"siteIds"`
		} `json:"categories"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	order := make([]navigation.CategoryOrder, len(request.Categories))
	for index, category := range request.Categories {
		order[index] = navigation.CategoryOrder{ID: category.ID, SiteIDs: category.SiteIDs}
	}
	revision, err := h.service.ReplaceContentOrder(
		r.Context(), navigationActor(r), chi.URLParam(r, "pageId"), request.ExpectedRevision, order,
	)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, map[string]int{"draftRevision": revision})
}

func (h *NavigationHandler) settings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.service.Settings(r.Context(), navigationActor(r), chi.URLParam(r, "pageId"))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, settings)
}

func (h *NavigationHandler) replaceSettings(w http.ResponseWriter, r *http.Request) {
	var request struct {
		ExpectedRevision int `json:"expectedRevision"`
		navigation.PageSettings
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	settings, err := h.service.ReplaceSettings(
		r.Context(), navigationActor(r), chi.URLParam(r, "pageId"), request.ExpectedRevision, request.PageSettings,
	)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, settings)
}

func (h *NavigationHandler) preview(w http.ResponseWriter, r *http.Request) {
	page, err := h.service.Preview(r.Context(), navigationActor(r), chi.URLParam(r, "pageId"), h.publicBaseURL)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, page)
}

func (h *NavigationHandler) publication(w http.ResponseWriter, r *http.Request) {
	publication, err := h.service.Publication(r.Context(), navigationActor(r), chi.URLParam(r, "pageId"), h.publicBaseURL)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, publication)
}

func (h *NavigationHandler) replacePublication(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Visibility     navigation.Visibility `json:"visibility"`
		Slug           string                `json:"slug"`
		ShowAuthor     bool                  `json:"showAuthor"`
		SEOTitle       string                `json:"seoTitle"`
		SEODescription string                `json:"seoDescription"`
		SEOImage       string                `json:"seoImage"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	publication, err := h.service.ReplacePublication(
		r.Context(), navigationActor(r), chi.URLParam(r, "pageId"), navigation.PublicationSettingsInput{
			Visibility: request.Visibility, Slug: request.Slug, ShowAuthor: request.ShowAuthor,
			SEOTitle: request.SEOTitle, SEODescription: request.SEODescription, SEOImage: request.SEOImage,
		}, h.publicBaseURL,
	)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, publication)
}

func (h *NavigationHandler) publish(w http.ResponseWriter, r *http.Request) {
	var request struct {
		ExpectedRevision int `json:"expectedRevision"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	if h.idempotency == nil {
		WriteError(w, r, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "发布幂等服务未配置", nil)
		return
	}
	session, _ := SessionFromContext(r.Context())
	pageID := chi.URLParam(r, "pageId")
	reservation, replay, err := h.idempotency.Begin(
		r.Context(), "publish:"+pageID, r.Header.Get("Idempotency-Key"), session.User.ID, request,
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
		var publication navigation.Publication
		if err := json.Unmarshal(replay.Data, &publication); err != nil {
			WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "读取幂等发布结果失败", nil)
			return
		}
		WriteJSON(w, r, replay.Status, publication)
		return
	}
	// Only Abort when Publish itself fails. After a successful side effect, never
	// Abort: clients would otherwise replay and create another snapshot.
	completed := false
	defer func() {
		if !completed {
			reservation.Abort(context.WithoutCancel(r.Context()))
		}
	}()
	publication, err := h.service.Publish(
		r.Context(), navigationActor(r), pageID, request.ExpectedRevision, h.publicBaseURL,
	)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	if err := reservation.Complete(r.Context(), http.StatusOK, publication); err != nil {
		// Publish already committed. Still return success so clients do not retry;
		// leave the in-progress key (no Abort) until natural expiry.
		slog.Warn("complete publish idempotency record", "error", err, "page_id", pageID)
	}
	completed = true
	WriteJSON(w, r, http.StatusOK, publication)
}

func (h *NavigationHandler) unpublish(w http.ResponseWriter, r *http.Request) {
	publication, err := h.service.Unpublish(r.Context(), navigationActor(r), chi.URLParam(r, "pageId"), h.publicBaseURL)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, publication)
}

func (h *NavigationHandler) publicHome(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if value, _, splitErr := net.SplitHostPort(r.Host); splitErr == nil {
		host = value
	}
	page, err := h.service.PublicHomeForHost(r.Context(), host)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writePublishedPage(w, r, page)
}

func (h *NavigationHandler) publicPage(w http.ResponseWriter, r *http.Request) {
	page, err := h.service.PublicBySlug(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writePublishedPage(w, r, page)
}

func writePublishedPage(w http.ResponseWriter, r *http.Request, page navigation.PublishedPage) {
	if r.Header.Get("If-None-Match") == page.ETag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("ETag", page.ETag)
	// Always revalidate against ETag so publish (theme/opacity/links) is visible
	// on the next refresh. max-age + stale-while-revalidate previously left
	// browsers serving pre-publish JSON for up to ~6 minutes.
	if page.Visibility == navigation.VisibilityPublic || page.Kind == navigation.PageKindSystem {
		w.Header().Set("Cache-Control", "public, max-age=0, must-revalidate")
	} else {
		w.Header().Set("Cache-Control", "private, no-cache")
	}
	WriteJSON(w, r, http.StatusOK, page)
}

func navigationActor(r *http.Request) navigation.Actor {
	session, _ := SessionFromContext(r.Context())
	return navigation.Actor{
		UserID: session.User.ID, Username: session.User.Username, AvatarURL: session.User.AvatarURL, Role: session.User.Role,
	}
}

func (h *NavigationHandler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, navigation.ErrForbidden):
		WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "没有该导航页的访问权限", nil)
	case errors.Is(err, navigation.ErrNotFound):
		WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "导航资源不存在", nil)
	case errors.Is(err, navigation.ErrPrecondition):
		WriteError(w, r, http.StatusPreconditionFailed, "DRAFT_REVISION_MISMATCH", "草稿版本已变化，请刷新后重试", nil)
	case errors.Is(err, navigation.ErrCategoryNotEmpty):
		WriteError(w, r, http.StatusConflict, "CATEGORY_NOT_EMPTY", "分类中仍有站点", nil)
	case errors.Is(err, navigation.ErrConflict), errors.Is(err, navigation.ErrInvalidOrder), errors.Is(err, navigation.ErrUncategorized):
		WriteError(w, r, http.StatusConflict, "CONFLICT", "导航内容发生冲突", nil)
	case errors.Is(err, navigation.ErrValidation):
		WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", err.Error(), nil)
	default:
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "导航操作失败", nil)
	}
}

func queryInt(r *http.Request, name string, fallback int) int {
	value, err := strconv.Atoi(r.URL.Query().Get(name))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
