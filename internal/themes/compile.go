package themes

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Package 是一个待编译的主题包：manifest + CSS + 资产。
// 三条来源（内置 embed、GitHub 导入、zip 上传）都收敛到这个类型。
type Package struct {
	Manifest Manifest
	CSS      []byte
	Assets   []Asset
}

// Compiled 是编译产物：不可变、自包含、可直接下发。
type Compiled struct {
	VersionID   string
	ContentHash string
	Manifest    Manifest
	CSS         []byte
	Assets      []Asset
}

// AssetBasePath 返回某个版本的资产同源前缀。
func AssetBasePath(versionID string) string {
	return "/api/v1/public/themes/" + versionID + "/assets/"
}

// Compile 把一个主题包编译成不可变版本。
//
// 顺序即失败顺序：体积 → 资产 → CSS 校验 → 资产引用交叉检查 → 拼接 →
// 哈希 → 版本 ID → 资产 URL 落地。每一步的错误都可以直接展示给主题作者。
func Compile(pkg Package, packageID string) (Compiled, error) {
	total := len(pkg.CSS)
	for _, asset := range pkg.Assets {
		total += len(asset.Data)
	}
	if total > MaxPackageBytes {
		return Compiled{}, invalidAsset("整包体积 %d 字节超过 %d 字节上限", total, MaxPackageBytes)
	}

	assets := make([]Asset, 0, len(pkg.Assets))
	available := make(map[string]bool, len(pkg.Assets))
	for _, asset := range pkg.Assets {
		validated, err := ValidateAsset(asset.Path, asset.Data)
		if err != nil {
			return Compiled{}, err
		}
		if available[validated.Path] {
			return Compiled{}, invalidAsset("资产路径 %s 重复", validated.Path)
		}
		available[validated.Path] = true
		assets = append(assets, validated)
	}
	sort.Slice(assets, func(i, j int) bool { return assets[i].Path < assets[j].Path })

	refs, err := validateCSSCollect(pkg.CSS, pkg.Manifest.FontFamilies())
	if err != nil {
		return Compiled{}, err
	}
	for _, ref := range refs {
		if !available[ref] {
			return Compiled{}, invalidCSS("CSS 引用了不存在的资产 %q", ref)
		}
	}

	families, err := collectFontFaceFamilies(pkg.CSS)
	if err != nil {
		return Compiled{}, err
	}
	themeCSS, err := CompileCSS(pkg.CSS, packageID)
	if err != nil {
		return Compiled{}, err
	}
	combined := TokensCSS(pkg.Manifest, packageID, families) + string(themeCSS)

	contentHash, err := contentHashOf(pkg.Manifest, combined, assets)
	if err != nil {
		return Compiled{}, err
	}
	versionID := "v" + contentHash[:32]

	// 哈希基于占位形式，因此版本 ID 不依赖自身；替换发生在定 ID 之后。
	final := strings.ReplaceAll(combined, AssetBasePlaceholder, AssetBasePath(versionID))

	return Compiled{
		VersionID:   versionID,
		ContentHash: contentHash,
		Manifest:    pkg.Manifest,
		CSS:         []byte(final),
		Assets:      assets,
	}, nil
}

// contentHashOf 覆盖 manifest、编译后的 CSS 与全部资产。资产按路径排序后
// 只计入路径与摘要，因此结果与 map 迭代顺序无关。
func contentHashOf(manifest Manifest, css string, assets []Asset) (string, error) {
	canonical, err := json.Marshal(manifest)
	if err != nil {
		return "", fmt.Errorf("canonicalise manifest: %w", err)
	}
	digest := sha256.New()
	digest.Write(canonical)
	digest.Write([]byte{0})
	digest.Write([]byte(css))
	digest.Write([]byte{0})
	for _, asset := range assets {
		fmt.Fprintf(digest, "%s\x00%s\n", asset.Path, asset.SHA256)
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}
