package httpapi

import (
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/yixian-huang/navax/internal/catalog"
	"github.com/yixian-huang/navax/internal/themeimport"
	"github.com/yixian-huang/navax/internal/themes"
)

// ThemeImportHandler 提供主题导入、卸载与 dry-run 校验。
type ThemeImportHandler struct {
	service        *themeimport.Service
	catalogService *catalog.Service
}

func NewThemeImportHandler(service *themeimport.Service, catalogService *catalog.Service) *ThemeImportHandler {
	return &ThemeImportHandler{service: service, catalogService: catalogService}
}

func (h *ThemeImportHandler) MountProtected(router chi.Router) {
	router.Post("/me/themes/import", h.importTheme)
	router.Delete("/me/themes/{themeId}", h.uninstall)
	router.Post("/themes/validate", h.validate)
	router.Post("/me/themes/{themeId}/check-update", h.checkUpdate)
}

const themeArchiveOverhead int64 = 1 << 20

type themeImportRequest struct {
	GitHubURL string `json:"githubUrl"`
	Ref       string `json:"ref"`
}

func (h *ThemeImportHandler) importTheme(w http.ResponseWriter, r *http.Request) {
	session, _ := SessionFromContext(r.Context())
	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	var (
		installed themes.InstalledTheme
		err       error
	)
	switch {
	case mediaType == "multipart/form-data":
		data, ok := h.readArchive(w, r)
		if !ok {
			return
		}
		installed, err = h.service.ImportZip(r.Context(), session.User.ID, data)
	case mediaType == "application/json":
		var payload themeImportRequest
		if !decodeJSON(w, r, &payload) {
			return
		}
		if payload.GitHubURL == "" {
			WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "githubUrl 不能为空", nil)
			return
		}
		installed, err = h.service.ImportGitHub(r.Context(), session.User.ID, payload.GitHubURL, payload.Ref)
	default:
		WriteError(w, r, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "请使用 multipart/form-data 上传 zip,或 application/json 提供 githubUrl", nil)
		return
	}
	if err != nil {
		h.writeImportError(w, r, err)
		return
	}
	// 201 返回与列表一致的 Theme 形状,前端免二次映射。回查失败或未命中时,
	// 宁可 500 也不要吐一个不满足 Theme schema 的裁剪响应——导入本身已经
	// 成功落库,客户端刷新主题列表即可看到,不需要靠这次响应兜底。
	items, listErr := h.catalogService.Themes(r.Context(), session.User.ID)
	if listErr == nil {
		for _, item := range items {
			if item.ID == installed.ThemeID {
				WriteJSON(w, r, http.StatusCreated, themeData(item))
				return
			}
		}
	}
	WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "主题已导入,但读取详情失败,请刷新主题列表", nil)
}

// readArchive 按 assets.go 的模式取 multipart 的 file 字段,压缩包硬上限。
func (h *ThemeImportHandler) readArchive(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	maximum := int64(themes.MaxArchiveBytes)
	r.Body = http.MaxBytesReader(w, r.Body, maximum+themeArchiveOverhead)
	if err := r.ParseMultipartForm(themeArchiveOverhead); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) || errors.Is(err, multipart.ErrMessageTooLarge) {
			WriteError(w, r, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "压缩包超过体积上限", nil)
			return nil, false
		}
		WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "multipart 上传内容无效", nil)
		return nil, false
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()
	file, header, err := r.FormFile("file")
	if err != nil {
		WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "file 字段必须提供且仅一次", nil)
		return nil, false
	}
	defer file.Close()
	if header.Size > maximum {
		WriteError(w, r, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "压缩包超过体积上限", nil)
		return nil, false
	}
	data, err := io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil || int64(len(data)) > maximum {
		WriteError(w, r, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "压缩包超过体积上限", nil)
		return nil, false
	}
	return data, true
}

func (h *ThemeImportHandler) writeImportError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, themes.ErrQuotaExceeded):
		WriteError(w, r, http.StatusConflict, "QUOTA_EXCEEDED", "私有主题数量已达上限(含已卸载但仍被历史发布引用的主题)", nil)
	case errors.Is(err, themes.ErrInvalidArchive), errors.Is(err, themes.ErrInvalidManifest),
		errors.Is(err, themes.ErrInvalidCSS), errors.Is(err, themes.ErrInvalidAsset):
		WriteError(w, r, http.StatusUnprocessableEntity, "THEME_INVALID", "主题包未通过校验", err)
	case errors.Is(err, themeimport.ErrHostNotAllowed):
		WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "仓库地址不被允许", err)
	case errors.Is(err, themeimport.ErrUpstream):
		WriteError(w, r, http.StatusBadGateway, "UPSTREAM_ERROR", "上游仓库拉取失败", err)
	default:
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "导入失败", nil)
	}
}

func (h *ThemeImportHandler) uninstall(w http.ResponseWriter, r *http.Request) {
	session, _ := SessionFromContext(r.Context())
	if _, err := h.service.Uninstall(r.Context(), session.User.ID, chi.URLParam(r, "themeId")); err != nil {
		if errors.Is(err, themes.ErrNotFound) {
			WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "主题不存在", nil)
			return
		}
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "卸载失败", nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ThemeImportHandler) checkUpdate(w http.ResponseWriter, r *http.Request) {
	session, _ := SessionFromContext(r.Context())
	status, err := h.service.CheckUpdate(r.Context(), session.User.ID, chi.URLParam(r, "themeId"))
	if err != nil {
		switch {
		case errors.Is(err, themes.ErrNotFound):
			WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "主题不存在", nil)
		case errors.Is(err, themeimport.ErrHostNotAllowed):
			WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "仓库地址不被允许", err)
		case errors.Is(err, themeimport.ErrUpstream):
			WriteError(w, r, http.StatusBadGateway, "UPSTREAM_ERROR", "上游仓库检查失败", err)
		default:
			WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "检查更新失败", nil)
		}
		return
	}
	WriteJSON(w, r, http.StatusOK, map[string]any{
		"sourceType": status.SourceType, "hasUpdate": status.HasUpdate,
		"currentSha": status.CurrentSha, "latestSha": status.LatestSha,
	})
}

func (h *ThemeImportHandler) validate(w http.ResponseWriter, r *http.Request) {
	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mediaType != "multipart/form-data" {
		WriteError(w, r, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "请使用 multipart/form-data 上传 zip", nil)
		return
	}
	data, ok := h.readArchive(w, r)
	if !ok {
		return
	}
	issues := h.service.ValidatePackage(data)
	if issues == nil {
		issues = []themeimport.ValidationIssue{}
	}
	WriteJSON(w, r, http.StatusOK, map[string]any{"valid": len(issues) == 0, "errors": issues})
}
