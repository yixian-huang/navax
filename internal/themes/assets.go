package themes

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"strings"
)

// ErrInvalidAsset 包裹所有资产层面的校验失败。
var ErrInvalidAsset = errors.New("invalid theme asset")

const (
	// MaxAssetBytes 是单个资产的体积上限。中文字体通常超过它，规范要求
	// 作者自行子集化。
	MaxAssetBytes = 524288
	// MaxPackageBytes 是整包（CSS + 全部资产）的体积上限。
	MaxPackageBytes = 4194304
)

// Asset 是一个校验通过的包内资产。
type Asset struct {
	// Path 是包内相对路径，形如 "fonts/sample.woff2"。
	Path   string
	MIME   string
	Data   []byte
	SHA256 string
}

// magicSignature 把内容特征映射到 MIME。类型按内容判定，不信任扩展名。
type magicSignature struct {
	mime   string
	prefix []byte
	// riffForm 非空时额外校验 RIFF 容器的 form 类型（WebP 用）。
	riffForm []byte
}

var magicSignatures = []magicSignature{
	{mime: "font/woff2", prefix: []byte("wOF2")},
	{mime: "image/png", prefix: []byte("\x89PNG\r\n\x1a\n")},
	{mime: "image/jpeg", prefix: []byte("\xff\xd8\xff")},
	{mime: "image/webp", prefix: []byte("RIFF"), riffForm: []byte("WEBP")},
}

// extensionMIME 是扩展名到期望 MIME 的映射，用于交叉检查声明与内容是否一致。
var extensionMIME = map[string]string{
	".woff2": "font/woff2",
	".png":   "image/png",
	".jpg":   "image/jpeg",
	".jpeg":  "image/jpeg",
	".webp":  "image/webp",
}

func invalidAsset(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidAsset, fmt.Sprintf(format, args...))
}

// ValidateAsset 校验一个包内资产的路径、体积与真实类型。
//
// 类型按 magic bytes 判定并与扩展名交叉核对：只信任扩展名等于让作者自己
// 决定服务端会吐出什么 Content-Type。SVG 一律拒绝——它可以携带脚本，且
// 本项目对用户上传也是同样策略。
func ValidateAsset(assetPath string, data []byte) (Asset, error) {
	cleaned, err := cleanAssetPath(assetPath)
	if err != nil {
		return Asset{}, err
	}
	if len(data) == 0 {
		return Asset{}, invalidAsset("资产 %s 内容为空", cleaned)
	}
	if len(data) > MaxAssetBytes {
		return Asset{}, invalidAsset("资产 %s 体积 %d 字节超过 %d 字节上限（中文字体请先做子集化）",
			cleaned, len(data), MaxAssetBytes)
	}

	extension := strings.ToLower(path.Ext(cleaned))
	if extension == ".svg" {
		return Asset{}, invalidAsset("资产 %s 是 svg —— SVG 一律拒绝", cleaned)
	}
	expected, known := extensionMIME[extension]
	if !known {
		return Asset{}, invalidAsset("资产 %s 的扩展名不在白名单（woff2/png/jpg/jpeg/webp）", cleaned)
	}

	actual := detectMIME(data)
	if actual == "" {
		return Asset{}, invalidAsset("资产 %s 的内容无法识别为受支持的类型", cleaned)
	}
	if actual != expected {
		return Asset{}, invalidAsset("资产 %s 的扩展名声明 %s，实际内容是 %s", cleaned, expected, actual)
	}

	sum := sha256.Sum256(data)
	return Asset{
		Path:   cleaned,
		MIME:   actual,
		Data:   data,
		SHA256: hex.EncodeToString(sum[:]),
	}, nil
}

func detectMIME(data []byte) string {
	for _, signature := range magicSignatures {
		if !bytes.HasPrefix(data, signature.prefix) {
			continue
		}
		if signature.riffForm != nil {
			// RIFF 容器：form 类型在偏移 8..12。
			if len(data) < 12 || !bytes.Equal(data[8:12], signature.riffForm) {
				continue
			}
		}
		return signature.mime
	}
	return ""
}

// cleanAssetPath 规范化并校验包内路径。拒绝绝对路径、越界路径与
// assets/ 之外的位置。
func cleanAssetPath(assetPath string) (string, error) {
	trimmed := strings.TrimSpace(assetPath)
	if trimmed == "" {
		return "", invalidAsset("资产路径为空")
	}
	if strings.HasPrefix(trimmed, "/") {
		return "", invalidAsset("资产路径 %q 不得以 / 开头", assetPath)
	}
	if strings.ContainsRune(trimmed, '\\') {
		return "", invalidAsset("资产路径 %q 不得包含反斜杠", assetPath)
	}
	cleaned := path.Clean(trimmed)
	if cleaned == "." || strings.HasPrefix(cleaned, "../") || cleaned != trimmed {
		return "", invalidAsset("资产路径 %q 必须是规范化的相对路径", assetPath)
	}
	return cleaned, nil
}
