package httpapi

import (
	"bytes"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/yixian-huang/navax/internal/themes"
)

// ThemeHandler 供应主题版本的编译产物与自带资产。
//
// 两个端点都是内容寻址的：versionID 由编译产物的哈希派生，因此同一个 URL
// 的字节永不改变，可以放心长缓存。撤销不依赖缓存清除——被撤销的版本不再
// 被任何快照引用，新访问根本不会请求它。
type ThemeHandler struct {
	store *themes.Store
}

func NewThemeHandler(store *themes.Store) *ThemeHandler {
	return &ThemeHandler{store: store}
}

func (h *ThemeHandler) MountPublic(router chi.Router) {
	router.Get("/public/themes/{versionId}.css", h.css)
	router.Head("/public/themes/{versionId}.css", h.css)
	router.Get("/public/themes/{versionId}/assets/*", h.asset)
	router.Head("/public/themes/{versionId}/assets/*", h.asset)
}

func (h *ThemeHandler) css(w http.ResponseWriter, r *http.Request) {
	versionID := chi.URLParam(r, "versionId")
	css, contentHash, status, err := h.store.VersionCSS(r.Context(), versionID)
	if err != nil {
		h.writeLookupError(w, r, err, "主题样式不存在")
		return
	}
	// 410 而不是 404：这个版本曾经存在，是被撤销的。区分开来，缓存层与
	// 排障的人都能看出差别。
	if status == themes.VersionStatusDisabled {
		WriteError(w, r, http.StatusGone, "GONE", "该主题版本已被下架", nil)
		return
	}
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	h.writeImmutableHeaders(w, contentHash)
	http.ServeContent(w, r, versionID+".css", time.Time{}, bytes.NewReader(css))
}

func (h *ThemeHandler) asset(w http.ResponseWriter, r *http.Request) {
	versionID := chi.URLParam(r, "versionId")
	assetPath := strings.TrimPrefix(chi.URLParam(r, "*"), "/")

	// 版本状态先查：被撤销的版本连资产也不再供应。
	_, _, status, err := h.store.VersionCSS(r.Context(), versionID)
	if err != nil {
		h.writeLookupError(w, r, err, "主题资产不存在")
		return
	}
	if status == themes.VersionStatusDisabled {
		WriteError(w, r, http.StatusGone, "GONE", "该主题版本已被下架", nil)
		return
	}

	asset, err := h.store.VersionAsset(r.Context(), versionID, assetPath)
	if err != nil {
		h.writeLookupError(w, r, err, "主题资产不存在")
		return
	}
	w.Header().Set("Content-Type", asset.MIME)
	w.Header().Set("Content-Length", strconv.Itoa(len(asset.Data)))
	h.writeImmutableHeaders(w, asset.SHA256)
	// 资产是主题作者提供的内容，按最严格的方式供应：不嗅探类型，禁止它
	// 被当作可执行文档解析。
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	http.ServeContent(w, r, assetPath, time.Time{}, bytes.NewReader(asset.Data))
}

func (h *ThemeHandler) writeImmutableHeaders(w http.ResponseWriter, etag string) {
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("ETag", `"`+etag+`"`)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cross-Origin-Resource-Policy", "same-site")
}

func (h *ThemeHandler) writeLookupError(w http.ResponseWriter, r *http.Request, err error, message string) {
	if errors.Is(err, themes.ErrNotFound) {
		WriteError(w, r, http.StatusNotFound, "NOT_FOUND", message, nil)
		return
	}
	WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "读取主题失败", nil)
}
