package common

import (
	"strings"
	"unicode"
)

// Content-policy / catalog helpers shared by agent engine, tools, and SSE handlers.
// Keep this package dependency-light so handler/session can import it without cycles.

// ContentPolicyMarkers are provider safety refusal substrings (lowercase match).
// Prefer specific phrases; avoid overly broad tokens that false-positive normal errors.
var ContentPolicyMarkers = []string{
	"content exists risk",
	"content_filter",
	"content filter",
	"sensitive content",
	"content management policy",
	"data_inspection_failed",
}

// IsContentPolicyMessage reports provider-side content safety refusals in free-form text.
func IsContentPolicyMessage(msg string) bool {
	if msg == "" {
		return false
	}
	lower := strings.ToLower(msg)
	for _, marker := range ContentPolicyMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

// catalogIntentNeedles are substrings that indicate the user wants a document
// inventory rather than deep document body reading.
var catalogIntentNeedles = []string{
	"有哪些知识", "有哪些文档", "有哪些资料", "有哪些文件",
	"有什么知识", "有什么文档", "有什么资料",
	"列出文档", "列出知识", "列出文件", "文档清单", "知识清单",
	"此知识库内有哪些", "这个知识库有哪些", "知识库里有哪些", "库里有哪些",
	"能查哪些库", "有哪些知识库",
	"what documents", "list documents", "list the documents",
	"what knowledge", "which documents", "document catalog", "list files in",
}

// catalogIntentExclusions — if present with a catalog needle, treat as content Q.
var catalogIntentExclusions = []string{
	"内容", "正文", "详情", "详细", "具体写", "讲了什么", "说了什么",
	"摘要", "总结一下里面", "解读", "翻译",
	"content of", "what does it say", "summarize the content", "explain in detail",
}

// IsCatalogIntent reports inventory/list questions that must not deep-read bodies.
func IsCatalogIntent(query string) bool {
	q := strings.TrimSpace(query)
	if q == "" {
		return false
	}
	lower := strings.ToLower(q)
	matched := false
	for _, needle := range catalogIntentNeedles {
		if strings.Contains(lower, strings.ToLower(needle)) {
			matched = true
			break
		}
	}
	if !matched {
		return false
	}
	for _, ex := range catalogIntentExclusions {
		if strings.Contains(lower, strings.ToLower(ex)) {
			return false
		}
	}
	return true
}

// PrefersChinese reports whether s is likely Chinese for reply localization.
func PrefersChinese(s string) bool {
	han := 0
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			han++
			if han >= 2 {
				return true
			}
		}
	}
	return false
}
