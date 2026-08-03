package knowledge_mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type hybridSearcher interface {
	HybridSearch(ctx context.Context, id string, params types.SearchParams) ([]*types.SearchResult, error)
}

type wikiQuerier interface {
	SearchPages(ctx context.Context, kbID string, query string, limit int) ([]*types.WikiPage, error)
	GetGraph(ctx context.Context, req *types.WikiGraphRequest) (*types.WikiGraphData, error)
}

type toolServices struct {
	scope *scopeResolver
	kb    hybridSearcher
	wiki  wikiQuerier
}

func registerTools(s *server.MCPServer, svc toolServices) {
	s.AddTool(mcp.NewTool(
		"kb_list",
		mcp.WithDescription("List knowledge bases authorized by the current API key. Each item includes wiki_enabled so agents can choose wiki tools correctly."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	), svc.handleKBList)

	s.AddTool(mcp.NewTool(
		"search_chunks",
		mcp.WithDescription("Hybrid (vector+keyword) retrieval across API-key-authorized knowledge bases. Optional kb_list intersects with the key scope (strict: any out-of-scope id fails). Returns merged chunks capped by limit (default 10, max 50)."),
		mcp.WithArray("kb_list", mcp.Description("Optional knowledge base IDs; must all be within API key scope"), mcp.WithStringItems()),
		mcp.WithString("query", mcp.Required(), mcp.Description("Natural-language search query")),
		mcp.WithNumber("limit", mcp.Description("Max chunks to return after merge (default 10, max 50)")),
		mcp.WithNumber("vector_threshold", mcp.Description("Optional minimum vector similarity 0..1")),
		mcp.WithNumber("keyword_threshold", mcp.Description("Optional minimum keyword score 0..1")),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	), svc.handleSearchChunks)

	s.AddTool(mcp.NewTool(
		"wiki_search",
		mcp.WithDescription("Full-text search over wiki-enabled knowledge bases in scope. Optional kb_list intersects with key scope; non-wiki KBs in the intersection are skipped. Fails with no_wiki_kb_in_scope when none remain."),
		mcp.WithArray("kb_list", mcp.Description("Optional knowledge base IDs; must all be within API key scope"), mcp.WithStringItems()),
		mcp.WithString("query", mcp.Required(), mcp.Description("Wiki page search query")),
		mcp.WithNumber("limit", mcp.Description("Max pages to return after merge (default 10, max 50)")),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	), svc.handleWikiSearch)

	s.AddTool(mcp.NewTool(
		"wiki_graph",
		mcp.WithDescription("Return wiki link graphs for wiki-enabled KBs in scope. Optional kb_list intersects with key scope. mode=overview|ego; ego requires center. limit is per-KB node cap (default 200, max 500)."),
		mcp.WithArray("kb_list", mcp.Description("Optional knowledge base IDs; must all be within API key scope"), mcp.WithStringItems()),
		mcp.WithString("mode", mcp.Description("overview (default) or ego")),
		mcp.WithString("center", mcp.Description("Center page slug when mode=ego")),
		mcp.WithNumber("depth", mcp.Description("Ego BFS depth (default 1, max 3)")),
		mcp.WithNumber("limit", mcp.Description("Max nodes per KB graph (default 200, max 500)")),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	), svc.handleWikiGraph)
}

func (s toolServices) handleKBList(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	authorized, err := s.scope.listAuthorized(ctx)
	if err != nil {
		return toolErrFrom(err), nil
	}
	return toolJSON(map[string]any{
		"knowledge_bases": toSummaries(authorized),
		"meta": map[string]any{
			"count": len(authorized),
		},
	})
}

func (s toolServices) handleSearchChunks(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query := strings.TrimSpace(req.GetString("query", ""))
	if query == "" {
		return toolErr(&ScopeError{Code: codeMissingQuery, Message: "query is required"}), nil
	}
	effective, err := s.scope.resolveEffective(ctx, req.GetStringSlice("kb_list", nil))
	if err != nil {
		return toolErrFrom(err), nil
	}
	_, hasLimit := req.GetArguments()["limit"]
	applied, requested, limErr := applyLimit(req.GetInt("limit", 0), hasLimit, defaultChunkLimit, maxChunkLimit)
	if limErr != nil {
		return toolErr(limErr), nil
	}

	vectorTh := req.GetFloat("vector_threshold", 0)
	keywordTh := req.GetFloat("keyword_threshold", 0)

	var merged []*types.SearchResult
	for _, kb := range effective {
		params := types.SearchParams{
			QueryText:        query,
			MatchCount:       applied,
			VectorThreshold:  vectorTh,
			KeywordThreshold: keywordTh,
		}
		results, searchErr := s.kb.HybridSearch(ctx, kb.ID, params)
		if searchErr != nil {
			return toolErrFrom(searchErr), nil
		}
		merged = append(merged, results...)
	}
	sort.SliceStable(merged, func(i, j int) bool {
		return merged[i].Score > merged[j].Score
	})
	if len(merged) > applied {
		merged = merged[:applied]
	}
	if merged == nil {
		merged = []*types.SearchResult{}
	}

	return toolJSON(map[string]any{
		"results": merged,
		"meta": map[string]any{
			"limit":          requested,
			"limit_applied":  applied,
			"count":          len(merged),
			"kb_count":       len(effective),
			"knowledge_bases": toSummaries(effective),
		},
	})
}

func (s toolServices) handleWikiSearch(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query := strings.TrimSpace(req.GetString("query", ""))
	if query == "" {
		return toolErr(&ScopeError{Code: codeMissingQuery, Message: "query is required"}), nil
	}
	effective, err := s.scope.resolveEffective(ctx, req.GetStringSlice("kb_list", nil))
	if err != nil {
		return toolErrFrom(err), nil
	}
	wikiKBs := filterWikiEnabled(effective)
	if len(wikiKBs) == 0 {
		return toolErr(&ScopeError{Code: codeNoWikiKBInScope, Message: "no wiki-enabled knowledge base in effective scope"}), nil
	}
	_, hasLimit := req.GetArguments()["limit"]
	applied, requested, limErr := applyLimit(req.GetInt("limit", 0), hasLimit, defaultWikiSearch, maxWikiSearch)
	if limErr != nil {
		return toolErr(limErr), nil
	}

	type pageHit struct {
		KBID    string           `json:"kb_id"`
		KBName  string           `json:"kb_name"`
		Page    *types.WikiPage  `json:"page"`
	}
	var hits []pageHit
	perKB := applied
	if perKB < 1 {
		perKB = 1
	}
	for _, kb := range wikiKBs {
		pages, searchErr := s.wiki.SearchPages(ctx, kb.ID, query, perKB)
		if searchErr != nil {
			return toolErrFrom(searchErr), nil
		}
		for _, p := range pages {
			hits = append(hits, pageHit{KBID: kb.ID, KBName: kb.Name, Page: p})
		}
	}
	if len(hits) > applied {
		hits = hits[:applied]
	}
	if hits == nil {
		hits = []pageHit{}
	}

	return toolJSON(map[string]any{
		"pages": hits,
		"meta": map[string]any{
			"limit":         requested,
			"limit_applied": applied,
			"count":         len(hits),
			"kb_count":      len(wikiKBs),
			"knowledge_bases": toSummaries(wikiKBs),
		},
	})
}

func (s toolServices) handleWikiGraph(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	effective, err := s.scope.resolveEffective(ctx, req.GetStringSlice("kb_list", nil))
	if err != nil {
		return toolErrFrom(err), nil
	}
	wikiKBs := filterWikiEnabled(effective)
	if len(wikiKBs) == 0 {
		return toolErr(&ScopeError{Code: codeNoWikiKBInScope, Message: "no wiki-enabled knowledge base in effective scope"}), nil
	}

	mode := strings.TrimSpace(req.GetString("mode", types.WikiGraphModeOverview))
	if mode == "" {
		mode = types.WikiGraphModeOverview
	}
	if mode != types.WikiGraphModeOverview && mode != types.WikiGraphModeEgo {
		return toolErr(&ScopeError{Code: codeInvalidMode, Message: "mode must be overview or ego"}), nil
	}
	center := strings.TrimSpace(req.GetString("center", ""))
	if mode == types.WikiGraphModeEgo && center == "" {
		return toolErr(&ScopeError{Code: codeMissingCenter, Message: "center is required when mode=ego"}), nil
	}

	_, hasLimit := req.GetArguments()["limit"]
	applied, requested, limErr := applyLimit(req.GetInt("limit", 0), hasLimit, defaultGraphLimit, maxGraphLimit)
	if limErr != nil {
		return toolErr(limErr), nil
	}

	depth := defaultGraphDepth
	if _, ok := req.GetArguments()["depth"]; ok {
		depth = req.GetInt("depth", defaultGraphDepth)
		if depth < 1 {
			return toolErr(&ScopeError{Code: codeInvalidLimit, Message: "depth must be a positive integer"}), nil
		}
		if depth > maxGraphDepth {
			depth = maxGraphDepth
		}
	}

	type graphEntry struct {
		KBID   string               `json:"kb_id"`
		KBName string               `json:"kb_name"`
		Graph  *types.WikiGraphData `json:"graph,omitempty"`
		Error  string               `json:"error,omitempty"`
	}
	graphs := make([]graphEntry, 0, len(wikiKBs))
	for _, kb := range wikiKBs {
		data, gErr := s.wiki.GetGraph(ctx, &types.WikiGraphRequest{
			KnowledgeBaseID: kb.ID,
			Mode:            mode,
			Center:          center,
			Depth:           depth,
			Limit:           applied,
		})
		entry := graphEntry{KBID: kb.ID, KBName: kb.Name}
		if gErr != nil {
			entry.Error = gErr.Error()
		} else {
			entry.Graph = data
		}
		graphs = append(graphs, entry)
	}

	return toolJSON(map[string]any{
		"graphs": graphs,
		"meta": map[string]any{
			"limit":         requested,
			"limit_applied": applied,
			"mode":          mode,
			"depth":         depth,
			"kb_count":      len(wikiKBs),
			"count":         len(graphs),
		},
	})
}

func toolJSON(v any) (*mcp.CallToolResult, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal result: %v", err)), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}

func toolErr(err *ScopeError) *mcp.CallToolResult {
	if err == nil {
		return mcp.NewToolResultError("unknown error")
	}
	return mcp.NewToolResultError(err.Error())
}

func toolErrFrom(err error) *mcp.CallToolResult {
	if err == nil {
		return mcp.NewToolResultError("unknown error")
	}
	if se, ok := err.(*ScopeError); ok {
		return toolErr(se)
	}
	return mcp.NewToolResultError(err.Error())
}
