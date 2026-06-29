package taxonomy

import (
	"strings"
	"unicode"
)

// NormalizeSlug returns a safe URL and path segment slug.
// NormalizeSlug 返回安全的 URL 与路径片段 slug。
func NormalizeSlug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastHyphen := false
	for _, r := range value {
		if isSlugRune(r) {
			builder.WriteRune(r)
			lastHyphen = false
			continue
		}
		if !lastHyphen {
			builder.WriteByte('-')
			lastHyphen = true
		}
	}
	return strings.Trim(builder.String(), "-")
}

// SlugFromName derives a safe slug from a display name.
// SlugFromName 根据展示名称生成安全 slug。
func SlugFromName(name string) string {
	return NormalizeSlug(name)
}

func isSlugRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-'
}

func normalizeName(value string) string {
	return strings.TrimSpace(value)
}

func hasPathTraversalSegment(path string) bool {
	for _, part := range strings.Split(path, "/") {
		if part == "" || part == "." || part == ".." {
			return true
		}
		for _, r := range part {
			if !isSlugRune(unicode.ToLower(r)) {
				return true
			}
		}
	}
	return false
}
