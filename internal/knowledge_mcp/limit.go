package knowledge_mcp

const (
	defaultChunkLimit = 10
	maxChunkLimit     = 50
	defaultWikiSearch = 10
	maxWikiSearch     = 50
	defaultGraphLimit = 200
	maxGraphLimit     = 500
	defaultGraphDepth = 1
	maxGraphDepth     = 3
)

func applyLimit(raw int, hasExplicit bool, def, max int) (applied int, requested int, err *ScopeError) {
	if !hasExplicit {
		return def, def, nil
	}
	if raw < 1 {
		return 0, raw, &ScopeError{Code: codeInvalidLimit, Message: "limit must be a positive integer"}
	}
	if raw > max {
		return max, raw, nil
	}
	return raw, raw, nil
}
