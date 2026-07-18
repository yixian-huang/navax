package httpapi

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/yixian-huang/navax/internal/dataexchange"
	"github.com/yixian-huang/navax/internal/navigation"
)

type DataExchangeHandler struct {
	service *dataexchange.Service
}

func NewDataExchangeHandler(service *dataexchange.Service) *DataExchangeHandler {
	return &DataExchangeHandler{service: service}
}

// MountProtected mounts the import and export routes. The caller must mount
// this handler inside the session-protected /api/v1 router group.
func (h *DataExchangeHandler) MountProtected(router chi.Router) {
	router.Post("/pages/{pageId}/imports/preview", h.previewImport)
	router.Post("/pages/{pageId}/imports", h.commitImport)
	router.Get("/pages/{pageId}/export", h.exportPage)
}

func (h *DataExchangeHandler) previewImport(w http.ResponseWriter, r *http.Request) {
	maximum, err := h.service.MaxUploadBytes(r.Context())
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	// Multipart boundaries and field headers need a small allowance beyond the
	// configured file limit; the file itself is independently bounded below.
	r.Body = http.MaxBytesReader(w, r.Body, maximum+(1<<20))
	if err := r.ParseMultipartForm(maximum + (1 << 20)); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			h.writeError(w, r, dataexchange.ErrPayloadTooLarge)
			return
		}
		WriteError(w, r, http.StatusUnprocessableEntity, "INVALID_IMPORT", "导入表单无效", err)
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		WriteError(w, r, http.StatusUnprocessableEntity, "INVALID_IMPORT", "必须上传导入文件", err)
		return
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil {
		WriteError(w, r, http.StatusUnprocessableEntity, "INVALID_IMPORT", "读取导入文件失败", err)
		return
	}
	if int64(len(content)) > maximum {
		h.writeError(w, r, dataexchange.ErrPayloadTooLarge)
		return
	}
	preview, err := h.service.Preview(
		r.Context(), navigationActor(r), chi.URLParam(r, "pageId"), strings.TrimSpace(r.FormValue("format")), content,
	)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, preview)
}

func (h *DataExchangeHandler) commitImport(w http.ResponseWriter, r *http.Request) {
	var request dataexchange.CommitInput
	if !decodeJSON(w, r, &request) {
		return
	}
	result, err := h.service.Commit(
		r.Context(), navigationActor(r), chi.URLParam(r, "pageId"), r.Header.Get("Idempotency-Key"), request,
	)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, result)
}

func (h *DataExchangeHandler) exportPage(w http.ResponseWriter, r *http.Request) {
	file, err := h.service.Export(
		r.Context(), navigationActor(r), chi.URLParam(r, "pageId"), strings.TrimSpace(r.URL.Query().Get("format")),
	)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": file.Filename})
	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", disposition)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(file.Content)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Content)
}

func (h *DataExchangeHandler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, dataexchange.ErrPayloadTooLarge):
		WriteError(w, r, http.StatusRequestEntityTooLarge, "IMPORT_TOO_LARGE", "导入文件超过实例限制", nil)
	case errors.Is(err, dataexchange.ErrImportExpired):
		WriteError(w, r, http.StatusConflict, "IMPORT_PREVIEW_EXPIRED", "导入预览已失效，请重新上传", nil)
	case errors.Is(err, navigation.ErrPrecondition):
		WriteError(w, r, http.StatusConflict, "DRAFT_REVISION_MISMATCH", "草稿版本已变化，请刷新后重新预览", nil)
	case errors.Is(err, dataexchange.ErrConflict), errors.Is(err, navigation.ErrConflict):
		// Prefer the Chinese detail from the service as the user-facing message.
		WriteError(w, r, http.StatusConflict, "IMPORT_CONFLICT", importConflictMessage(err), nil)
	case errors.Is(err, dataexchange.ErrValidation), errors.Is(err, navigation.ErrValidation):
		WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", err.Error(), nil)
	case errors.Is(err, navigation.ErrForbidden):
		WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "没有该导航页的访问权限", nil)
	case errors.Is(err, navigation.ErrNotFound):
		WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "导航页不存在", nil)
	default:
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "导入或导出操作失败", nil)
	}
}

// importConflictMessage extracts a short Chinese explanation from wrapped conflict errors.
func importConflictMessage(err error) string {
	if err == nil {
		return "导入内容发生冲突"
	}
	text := err.Error()
	for _, prefix := range []string{
		dataexchange.ErrConflict.Error() + ": ",
		navigation.ErrConflict.Error() + ": ",
	} {
		if rest, ok := strings.CutPrefix(text, prefix); ok && strings.TrimSpace(rest) != "" {
			return rest
		}
	}
	// Fallback when only the sentinel is returned.
	if errors.Is(err, dataexchange.ErrConflict) || errors.Is(err, navigation.ErrConflict) {
		if text != dataexchange.ErrConflict.Error() && text != navigation.ErrConflict.Error() {
			return text
		}
	}
	return "导入内容发生冲突"
}
