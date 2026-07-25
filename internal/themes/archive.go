package themes

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
)

// 解包硬限。压缩输入与解压总量共用同一上限:它是包体上限(4 MiB)的 4 倍,
// 给 README、LICENSE 这类会被忽略但仍占解压预算的文件留余量。
const (
	MaxArchiveBytes = 16 << 20
	MaxArchiveFiles = 200
)

// ErrInvalidArchive 表示压缩包本身不可接受(路径逃逸、超限、格式损坏)。
// 与 ErrInvalidManifest/ErrInvalidCSS 并列,是导入错误分类的 archive 阶段。
var ErrInvalidArchive = errors.New("invalid theme archive")

func invalidArchive(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidArchive, fmt.Sprintf(format, args...))
}

// ExtractZip 把 zip 解为路径→内容,应用全部防护。
func ExtractZip(data []byte) (map[string][]byte, error) {
	if len(data) > MaxArchiveBytes {
		return nil, invalidArchive("压缩包超过 %d 字节上限", MaxArchiveBytes)
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, invalidArchive("无法解析 zip: %v", err)
	}
	files := map[string][]byte{}
	var total int
	for _, entry := range reader.File {
		if entry.FileInfo().IsDir() {
			continue
		}
		name, err := cleanArchivePath(entry.Name)
		if err != nil {
			return nil, err
		}
		if len(files) >= MaxArchiveFiles {
			return nil, invalidArchive("文件数超过 %d 上限", MaxArchiveFiles)
		}
		rc, err := entry.Open()
		if err != nil {
			return nil, invalidArchive("无法读取 %s: %v", name, err)
		}
		content, err := readCapped(rc, &total)
		rc.Close()
		if err != nil {
			return nil, err
		}
		files[name] = content
	}
	return stripTopLevelDir(files), nil
}

// ExtractTarGz 解 gzip tarball(GitHub codeload 产物),拒绝一切链接与设备条目。
func ExtractTarGz(data []byte) (map[string][]byte, error) {
	if len(data) > MaxArchiveBytes {
		return nil, invalidArchive("压缩包超过 %d 字节上限", MaxArchiveBytes)
	}
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, invalidArchive("无法解析 gzip: %v", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	files := map[string][]byte{}
	var total int
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, invalidArchive("无法解析 tar: %v", err)
		}
		switch header.Typeflag {
		case tar.TypeDir, tar.TypeXGlobalHeader, tar.TypeXHeader:
			continue
		case tar.TypeReg:
		default:
			return nil, invalidArchive("拒绝非普通文件条目 %s(链接/设备)", header.Name)
		}
		name, err := cleanArchivePath(header.Name)
		if err != nil {
			return nil, err
		}
		if len(files) >= MaxArchiveFiles {
			return nil, invalidArchive("文件数超过 %d 上限", MaxArchiveFiles)
		}
		content, err := readCapped(tr, &total)
		if err != nil {
			return nil, err
		}
		files[name] = content
	}
	return stripTopLevelDir(files), nil
}

// readCapped 累计解压总量,越过 MaxArchiveBytes 立即失败——不是读完再量。
func readCapped(r io.Reader, total *int) ([]byte, error) {
	remaining := MaxArchiveBytes - *total
	content, err := io.ReadAll(io.LimitReader(r, int64(remaining)+1))
	if err != nil {
		return nil, invalidArchive("读取失败: %v", err)
	}
	if len(content) > remaining {
		return nil, invalidArchive("解压总量超过 %d 字节上限", MaxArchiveBytes)
	}
	*total += len(content)
	return content, nil
}

// cleanArchivePath 拒绝绝对路径、反斜杠与 .. 逃逸(zip-slip)。
func cleanArchivePath(name string) (string, error) {
	if strings.Contains(name, `\`) || strings.HasPrefix(name, "/") {
		return "", invalidArchive("非法路径 %q", name)
	}
	cleaned := path.Clean(name)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || cleaned != name {
		return "", invalidArchive("非法路径 %q", name)
	}
	return cleaned, nil
}

// stripTopLevelDir:全部条目共享单一顶层目录时剥掉一层(GitHub tarball 的
// {repo}-{sha}/ 前缀与作者打 zip 时的习惯目录)。
func stripTopLevelDir(files map[string][]byte) map[string][]byte {
	var prefix string
	for name := range files {
		idx := strings.IndexByte(name, '/')
		if idx <= 0 {
			return files
		}
		dir := name[:idx+1]
		if prefix == "" {
			prefix = dir
		} else if dir != prefix {
			return files
		}
	}
	if prefix == "" {
		return files
	}
	stripped := make(map[string][]byte, len(files))
	for name, data := range files {
		stripped[strings.TrimPrefix(name, prefix)] = data
	}
	return stripped
}

// PackageFromFiles 从解包结果组装 Package:白名单提取 theme.json / theme.css /
// assets/** / preview.png,其余文件(README、LICENSE、.github/ 等)忽略。
func PackageFromFiles(files map[string][]byte) (Package, error) {
	manifestData, ok := files["theme.json"]
	if !ok {
		return Package{}, invalidArchive("缺少 theme.json")
	}
	manifest, err := ParseManifest(manifestData)
	if err != nil {
		return Package{}, err
	}
	pkg := Package{Manifest: manifest}
	if css, ok := files["theme.css"]; ok {
		pkg.CSS = css
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names) // 资产顺序确定性,与内容哈希稳定性一致
	for _, name := range names {
		var assetPath string
		switch {
		case strings.HasPrefix(name, "assets/"):
			assetPath = strings.TrimPrefix(name, "assets/")
		case name == "preview.png":
			assetPath = "preview.png"
		default:
			continue
		}
		asset, err := ValidateAsset(assetPath, files[name])
		if err != nil {
			return Package{}, err
		}
		pkg.Assets = append(pkg.Assets, asset)
	}
	return pkg, nil
}
