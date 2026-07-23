package themes

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// builtinFS 承载随二进制分发的首批内置主题。每个子目录是一个完整的主题包：
//
//	builtin/<id>/theme.json   必需
//	builtin/<id>/theme.css    可选
//	builtin/<id>/assets/…     可选
//
// 它们和第三方包走完全相同的解析、校验与编译路径——内置不等于免检。
//
//go:embed builtin
var builtinFS embed.FS

const builtinRoot = "builtin"

// BuiltinPackages 解析全部内置主题包。
//
// 结果按包 ID 升序返回：内置主题会被写入数据库并参与内容哈希，顺序必须与
// embed FS 的遍历顺序无关，否则同一份二进制在不同平台上可能算出不同结果。
func BuiltinPackages() ([]Package, error) {
	entries, err := fs.ReadDir(builtinFS, builtinRoot)
	if err != nil {
		return nil, fmt.Errorf("read builtin themes: %w", err)
	}

	packages := make([]Package, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pkg, err := readBuiltinPackage(entry.Name())
		if err != nil {
			return nil, err
		}
		packages = append(packages, pkg)
	}
	sort.Slice(packages, func(i, j int) bool {
		return packages[i].Manifest.ID < packages[j].Manifest.ID
	})
	return packages, nil
}

// readBuiltinPackage 读取单个内置主题目录。
func readBuiltinPackage(id string) (Package, error) {
	dir := path.Join(builtinRoot, id)

	manifestData, err := fs.ReadFile(builtinFS, path.Join(dir, "theme.json"))
	if err != nil {
		return Package{}, fmt.Errorf("builtin theme %s: %w", id, err)
	}
	manifest, err := ParseManifest(manifestData)
	if err != nil {
		return Package{}, fmt.Errorf("builtin theme %s: %w", id, err)
	}
	// 目录名即包 ID：内置主题的加载路径与 slug 必须一一对应，否则两份包
	// 可能声明同一个 ID 而只有一份被装载。
	if manifest.ID != id {
		return Package{}, fmt.Errorf("builtin theme %s: theme.json 的 id 为 %q，必须与目录名一致", id, manifest.ID)
	}

	css, err := fs.ReadFile(builtinFS, path.Join(dir, "theme.css"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Package{}, fmt.Errorf("builtin theme %s: %w", id, err)
	}

	assets, err := readBuiltinAssets(dir)
	if err != nil {
		return Package{}, fmt.Errorf("builtin theme %s: %w", id, err)
	}

	return Package{Manifest: manifest, CSS: css, Assets: assets}, nil
}

// readBuiltinAssets 读取可选的 assets/ 子目录。路径以包为根（形如
// "fonts/x.woff2"，对应磁盘上的 assets/fonts/x.woff2），与 CSS 里的
// url("asset:…") 引用同一套写法。
func readBuiltinAssets(dir string) ([]Asset, error) {
	root := path.Join(dir, "assets")
	if _, err := fs.Stat(builtinFS, root); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var assets []Asset
	err := fs.WalkDir(builtinFS, root, func(entryPath string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, readErr := fs.ReadFile(builtinFS, entryPath)
		if readErr != nil {
			return readErr
		}
		// 路径相对包内 assets/ 目录：CSS 里写 url("asset:fonts/x.woff2")
		// 就对应 assets/fonts/x.woff2。三条来源（embed、zip、GitHub）必须
		// 用同一套形状，否则同一份 CSS 换个来源就找不到资产。
		asset, validateErr := ValidateAsset(strings.TrimPrefix(entryPath, root+"/"), data)
		if validateErr != nil {
			return validateErr
		}
		assets = append(assets, asset)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(assets, func(i, j int) bool { return assets[i].Path < assets[j].Path })
	return assets, nil
}
