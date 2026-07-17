package backgrounds

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "image/gif"
	_ "image/png"

	_ "golang.org/x/image/webp"
)

type processedFile struct {
	Path       string
	Filename   string
	MIMEType   string
	MediaKind  MediaKind
	Width      int
	Height     int
	DurationMS int
	PosterURL  string // filled after asset upload of poster when video; empty here, set in service if we upload poster first
	PosterPath string // local path to poster image for video
}

func processUpload(ctx context.Context, workRoot, filename, declaredMIME string, body io.Reader) (processedFile, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	mime := strings.ToLower(strings.TrimSpace(strings.Split(declaredMIME, ";")[0]))
	if mime == "" {
		mime = mimeFromExt(ext)
	}

	tmp, err := os.CreateTemp(workRoot, "bg-upload-*")
	if err != nil {
		return processedFile{}, err
	}
	tmpPath := tmp.Name()
	if _, err := io.Copy(tmp, body); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return processedFile{}, err
	}
	_ = tmp.Close()

	if isVideoMIME(mime) || isVideoExt(ext) {
		out, err := processVideo(ctx, workRoot, tmpPath)
		_ = os.Remove(tmpPath)
		return out, err
	}
	out, err := processImage(workRoot, tmpPath, filename, mime)
	if err != nil {
		_ = os.Remove(tmpPath)
		return processedFile{}, err
	}
	// processImage may replace file; remove original staging if different
	if out.Path != tmpPath {
		_ = os.Remove(tmpPath)
	}
	return out, nil
}

func processImage(workRoot, srcPath, filename, mime string) (processedFile, error) {
	f, err := os.Open(srcPath)
	if err != nil {
		return processedFile{}, err
	}
	cfg, format, err := image.DecodeConfig(f)
	_ = f.Close()
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return processedFile{}, ErrInvalidFile
	}
	if cfg.Width < 64 || cfg.Height < 64 {
		return processedFile{}, fmt.Errorf("%w: background image too small", ErrInvalidFile)
	}

	// GIF / already-small WebP: keep original bytes (animation-safe).
	if format == "gif" || (format == "webp" && fileSize(srcPath) < 2<<20) {
		return processedFile{
			Path: srcPath, Filename: filename, MIMEType: mimeFromFormat(format),
			MediaKind: MediaImage, Width: cfg.Width, Height: cfg.Height,
		}, nil
	}

	f, err = os.Open(srcPath)
	if err != nil {
		return processedFile{}, err
	}
	img, format, err := image.Decode(f)
	_ = f.Close()
	if err != nil {
		return processedFile{}, ErrInvalidFile
	}

	// Re-encode without changing dimensions (quality/size only).
	out, err := os.CreateTemp(workRoot, "bg-img-*.jpg")
	if err != nil {
		return processedFile{}, err
	}
	outPath := out.Name()
	if err := jpeg.Encode(out, img, &jpeg.Options{Quality: TargetJPEGQuality}); err != nil {
		_ = out.Close()
		_ = os.Remove(outPath)
		// fallback keep original
		return processedFile{
			Path: srcPath, Filename: filename, MIMEType: mimeFromFormat(format),
			MediaKind: MediaImage, Width: cfg.Width, Height: cfg.Height,
		}, nil
	}
	_ = out.Close()

	// If re-encode is larger, keep original.
	if fileSize(outPath) >= fileSize(srcPath) && format != "png" {
		_ = os.Remove(outPath)
		return processedFile{
			Path: srcPath, Filename: ensureExt(filename, mimeFromFormat(format)),
			MIMEType: mimeFromFormat(format), MediaKind: MediaImage,
			Width: cfg.Width, Height: cfg.Height,
		}, nil
	}
	// Prefer JPEG when we re-encoded (even from PNG) for size.
	_ = os.Remove(srcPath)
	return processedFile{
		Path: outPath, Filename: "background.jpg", MIMEType: "image/jpeg",
		MediaKind: MediaImage, Width: cfg.Width, Height: cfg.Height,
	}, nil
}

func processVideo(ctx context.Context, workRoot, srcPath string) (processedFile, error) {
	if !FFmpegAvailable() {
		return processedFile{}, ErrFFmpegRequired
	}
	// Probe duration / size
	meta, err := ffprobe(ctx, srcPath)
	if err != nil {
		return processedFile{}, fmt.Errorf("%w: %v", ErrInvalidFile, err)
	}
	if meta.DurationSec > MaxVideoSeconds+0.5 {
		return processedFile{}, ErrVideoTooLong
	}

	outPath := filepath.Join(workRoot, fmt.Sprintf("bg-vid-%d.mp4", time.Now().UnixNano()))
	// Scale only if longer edge exceeds MaxVideoEdgePx; else keep resolution (-2 keeps aspect).
	scaleFilter := fmt.Sprintf("scale='min(%d,iw)':'min(%d,ih)':force_original_aspect_ratio=decrease", MaxVideoEdgePx, MaxVideoEdgePx)
	args := []string{
		"-y", "-i", srcPath,
		"-an",
		"-c:v", "libx264", "-preset", "veryfast", "-crf", "23",
		"-pix_fmt", "yuv420p",
		"-movflags", "+faststart",
		"-vf", scaleFilter,
		"-t", strconv.Itoa(MaxVideoSeconds),
		outPath,
	}
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		_ = os.Remove(outPath)
		return processedFile{}, fmt.Errorf("%w: ffmpeg: %s", ErrInvalidFile, strings.TrimSpace(stderr.String()))
	}

	posterPath := filepath.Join(workRoot, fmt.Sprintf("bg-poster-%d.jpg", time.Now().UnixNano()))
	posterArgs := []string{"-y", "-i", outPath, "-vframes", "1", "-q:v", "3", posterPath}
	if err := exec.CommandContext(ctx, "ffmpeg", posterArgs...).Run(); err != nil {
		_ = os.Remove(posterPath)
		posterPath = ""
	}

	w, h := meta.Width, meta.Height
	if w > MaxVideoEdgePx || h > MaxVideoEdgePx {
		// approximate after scale
		if w >= h {
			h = h * MaxVideoEdgePx / w
			w = MaxVideoEdgePx
		} else {
			w = w * MaxVideoEdgePx / h
			h = MaxVideoEdgePx
		}
	}
	durMS := int(meta.DurationSec * 1000)
	if durMS <= 0 {
		durMS = 1000
	}
	return processedFile{
		Path: outPath, Filename: "background.mp4", MIMEType: "video/mp4",
		MediaKind: MediaVideo, Width: w, Height: h, DurationMS: durMS, PosterPath: posterPath,
	}, nil
}

type probeMeta struct {
	DurationSec float64
	Width       int
	Height      int
}

func ffprobe(ctx context.Context, path string) (probeMeta, error) {
	if _, err := exec.LookPath("ffprobe"); err != nil {
		// duration from ffmpeg -i
		return probeMeta{DurationSec: 5, Width: 1280, Height: 720}, nil
	}
	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "error", "-select_streams", "v:0",
		"-show_entries", "stream=width,height:format=duration",
		"-of", "json", path)
	out, err := cmd.Output()
	if err != nil {
		return probeMeta{}, err
	}
	var parsed struct {
		Streams []struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return probeMeta{}, err
	}
	meta := probeMeta{Width: 1280, Height: 720}
	if len(parsed.Streams) > 0 {
		meta.Width = parsed.Streams[0].Width
		meta.Height = parsed.Streams[0].Height
	}
	if d, err := strconv.ParseFloat(parsed.Format.Duration, 64); err == nil {
		meta.DurationSec = d
	}
	return meta, nil
}

func clearBackgroundInSettingsJSON(settingsJSON, mediaURL, mediaID string) (string, bool) {
	var root map[string]any
	if err := json.Unmarshal([]byte(settingsJSON), &root); err != nil {
		return settingsJSON, false
	}
	appearance, _ := root["appearance"].(map[string]any)
	if appearance == nil {
		return settingsJSON, false
	}
	bg, _ := appearance["background"].(map[string]any)
	if bg == nil {
		return settingsJSON, false
	}
	value, _ := bg["value"].(string)
	mid, _ := bg["mediaId"].(string)
	if value != mediaURL && mid != mediaID && !strings.Contains(value, mediaID) {
		return settingsJSON, false
	}
	appearance["background"] = map[string]any{
		"type": "none", "value": "", "opacity": 1,
	}
	root["appearance"] = appearance
	out, err := json.Marshal(root)
	if err != nil {
		return settingsJSON, false
	}
	return string(out), true
}

func isVideoMIME(m string) bool {
	return strings.HasPrefix(m, "video/")
}
func isVideoExt(ext string) bool {
	switch ext {
	case ".mp4", ".webm", ".mov", ".m4v":
		return true
	default:
		return false
	}
}
func mimeFromExt(ext string) string {
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".mp4", ".m4v":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	default:
		return "application/octet-stream"
	}
}
func mimeFromFormat(format string) string {
	switch format {
	case "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "gif":
		return "image/gif"
	case "webp":
		return "image/webp"
	default:
		return "image/jpeg"
	}
}
func ensureExt(name, mime string) string {
	if filepath.Ext(name) != "" {
		return name
	}
	switch mime {
	case "image/jpeg":
		return name + ".jpg"
	case "image/png":
		return name + ".png"
	default:
		return name
	}
}
func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}
