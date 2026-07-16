package dataexchange

import (
	"encoding/json"
	"fmt"
	stdhtml "html"
	"net/url"
	"regexp"
	"strings"
)

type parsedCategory struct {
	SourceID string
	Name     string
	Sites    []parsedSite
}

type parsedSite struct {
	SourceID string
	Title    string
	URL      string
}

var (
	h3Pattern     = regexp.MustCompile(`(?i)<h3\b[^>]*>(.*?)</h3>`)
	anchorPattern = regexp.MustCompile(`(?i)<a\b([^>]*)>(.*?)</a>`)
	hrefPattern   = regexp.MustCompile(`(?i)\bhref\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))`)
	tagPattern    = regexp.MustCompile(`<[^>]+>`)
)

func parseImport(format string, content []byte) ([]parsedCategory, error) {
	switch format {
	case FormatBookmarksHTML:
		return parseBookmarksHTML(string(content))
	case FormatNavaxJSON:
		return parseNavaxJSON(content)
	default:
		return nil, fmt.Errorf("%w: format must be bookmarks-html or navax-json", ErrValidation)
	}
}

func parseBookmarksHTML(content string) ([]parsedCategory, error) {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	categories := make([]parsedCategory, 0)
	categoryIndex := make(map[string]int)
	folders := make([]string, 0)
	dlFrames := make([]bool, 0)
	pendingFolder := ""
	siteNumber := 0

	ensureCategory := func(name string) int {
		name = strings.TrimSpace(name)
		if name == "" {
			name = "未分类"
		}
		key := strings.ToLower(name)
		if index, ok := categoryIndex[key]; ok {
			return index
		}
		index := len(categories)
		categories = append(categories, parsedCategory{
			SourceID: fmt.Sprintf("category-%06d", index+1), Name: name, Sites: make([]parsedSite, 0),
		})
		categoryIndex[key] = index
		return index
	}

	for _, line := range lines {
		if match := h3Pattern.FindStringSubmatch(line); len(match) == 2 {
			pendingFolder = cleanHTMLText(match[1])
		}
		lower := strings.ToLower(line)
		for offset := 0; ; {
			relative := strings.Index(lower[offset:], "<dl")
			if relative < 0 {
				break
			}
			offset += relative + 3
			pushed := pendingFolder != ""
			if pushed {
				folders = append(folders, pendingFolder)
				pendingFolder = ""
			}
			dlFrames = append(dlFrames, pushed)
		}

		for _, match := range anchorPattern.FindAllStringSubmatch(line, -1) {
			if len(match) != 3 {
				continue
			}
			href := hrefPattern.FindStringSubmatch(match[1])
			if len(href) != 4 {
				continue
			}
			rawURL := href[1]
			if rawURL == "" {
				rawURL = href[2]
			}
			if rawURL == "" {
				rawURL = href[3]
			}
			categoryName := "未分类"
			if len(folders) > 0 {
				categoryName = folders[len(folders)-1]
			}
			siteNumber++
			index := ensureCategory(categoryName)
			categories[index].Sites = append(categories[index].Sites, parsedSite{
				SourceID: fmt.Sprintf("site-%06d", siteNumber),
				Title:    cleanHTMLText(match[2]), URL: stdhtml.UnescapeString(strings.TrimSpace(rawURL)),
			})
		}

		closeCount := strings.Count(lower, "</dl>")
		for range closeCount {
			if len(dlFrames) == 0 {
				break
			}
			last := len(dlFrames) - 1
			if dlFrames[last] && len(folders) > 0 {
				folders = folders[:len(folders)-1]
			}
			dlFrames = dlFrames[:last]
		}
	}

	if siteNumber == 0 && !strings.Contains(strings.ToLower(content), "netscape-bookmark-file-1") {
		return nil, fmt.Errorf("%w: bookmark file contains no links", ErrValidation)
	}
	return categories, nil
}

func cleanHTMLText(value string) string {
	return strings.TrimSpace(stdhtml.UnescapeString(tagPattern.ReplaceAllString(value, "")))
}

func parseNavaxJSON(content []byte) ([]parsedCategory, error) {
	var document struct {
		Format  string `json:"format"`
		Version int    `json:"version"`
		Page    struct {
			Categories []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"categories"`
			Sites []struct {
				ID         string `json:"id"`
				CategoryID string `json:"categoryId"`
				Title      string `json:"title"`
				URL        string `json:"url"`
			} `json:"sites"`
		} `json:"page"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(content)))
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("%w: decode navax JSON: %v", ErrValidation, err)
	}
	if document.Format != "navax-export" || document.Version != 1 {
		return nil, fmt.Errorf("%w: unsupported navax export format or version", ErrValidation)
	}

	categories := make([]parsedCategory, 0, len(document.Page.Categories)+1)
	byID := make(map[string]int, len(document.Page.Categories))
	usedCategoryIDs := make(map[string]struct{})
	for index, source := range document.Page.Categories {
		sourceID := uniqueSourceID(source.ID, fmt.Sprintf("category-%06d", index+1), usedCategoryIDs)
		byID[source.ID] = len(categories)
		categories = append(categories, parsedCategory{SourceID: sourceID, Name: strings.TrimSpace(source.Name), Sites: make([]parsedSite, 0)})
	}
	uncategorized := -1
	usedSiteIDs := make(map[string]struct{})
	for index, source := range document.Page.Sites {
		categoryIndex, ok := byID[source.CategoryID]
		if !ok {
			if uncategorized < 0 {
				uncategorized = len(categories)
				categories = append(categories, parsedCategory{
					SourceID: uniqueSourceID("uncategorized", fmt.Sprintf("category-%06d", len(categories)+1), usedCategoryIDs),
					Name:     "未分类", Sites: make([]parsedSite, 0),
				})
			}
			categoryIndex = uncategorized
		}
		sourceID := uniqueSourceID(source.ID, fmt.Sprintf("site-%06d", index+1), usedSiteIDs)
		categories[categoryIndex].Sites = append(categories[categoryIndex].Sites, parsedSite{
			SourceID: sourceID, Title: strings.TrimSpace(source.Title), URL: strings.TrimSpace(source.URL),
		})
	}
	return categories, nil
}

func uniqueSourceID(candidate, fallback string, used map[string]struct{}) string {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		candidate = fallback
	}
	base := candidate
	for suffix := 2; ; suffix++ {
		if _, exists := used[candidate]; !exists {
			used[candidate] = struct{}{}
			return candidate
		}
		candidate = fmt.Sprintf("%s-%d", base, suffix)
	}
}

func cleanHTTPURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil {
		return "", fmt.Errorf("URL 必须是无凭据的绝对 HTTP(S) 地址")
	}
	if len(parsed.String()) > 2048 {
		return "", fmt.Errorf("URL 长度不能超过 2048")
	}
	return parsed.String(), nil
}
