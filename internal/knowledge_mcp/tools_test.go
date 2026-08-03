package knowledge_mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/mark3labs/mcp-go/mcp"
)

type fakeHybrid struct {
	byKB map[string][]*types.SearchResult
	err  error
}

func (f *fakeHybrid) HybridSearch(_ context.Context, id string, _ types.SearchParams) ([]*types.SearchResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.byKB[id], nil
}

type fakeWiki struct {
	pages map[string][]*types.WikiPage
	graph *types.WikiGraphData
	err   error
}

func (f *fakeWiki) SearchPages(_ context.Context, kbID string, _ string, _ int) ([]*types.WikiPage, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.pages[kbID], nil
}

func (f *fakeWiki) GetGraph(_ context.Context, _ *types.WikiGraphRequest) (*types.WikiGraphData, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.graph, nil
}

func TestHandleSearchChunks_MergesAndCaps(t *testing.T) {
	svc := toolServices{
		scope: newScopeResolver(&listOnlyKB{list: []*types.KnowledgeBase{
			{ID: "a", Name: "A"},
			{ID: "b", Name: "B"},
		}}),
		kb: &fakeHybrid{byKB: map[string][]*types.SearchResult{
			"a": {{ID: "1", Score: 0.5}},
			"b": {{ID: "2", Score: 0.9}, {ID: "3", Score: 0.8}},
		}},
	}
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"query": "q", "limit": 2}
	res, err := svc.handleSearchChunks(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("tool error: %#v", res.Content)
	}
	text := toolText(t, res)
	if !strings.Contains(text, `"count": 2`) {
		t.Fatalf("unexpected: %s", text)
	}
	if !strings.Contains(text, `"id": "2"`) {
		t.Fatalf("expected highest score first: %s", text)
	}
}

func TestHandleWikiSearch_NoWiki(t *testing.T) {
	svc := toolServices{
		scope: newScopeResolver(&listOnlyKB{list: []*types.KnowledgeBase{
			{ID: "a", Name: "A", IndexingStrategy: types.IndexingStrategy{WikiEnabled: false}},
		}}),
		wiki: &fakeWiki{},
	}
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"query": "q"}
	res, err := svc.handleWikiSearch(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Fatal("expected tool error")
	}
	text := toolText(t, res)
	if !strings.Contains(text, codeNoWikiKBInScope) {
		t.Fatalf("got %s", text)
	}
}

func TestHandleWikiGraph_MixedWiki(t *testing.T) {
	svc := toolServices{
		scope: newScopeResolver(&listOnlyKB{list: []*types.KnowledgeBase{
			{ID: "a", Name: "A", IndexingStrategy: types.IndexingStrategy{WikiEnabled: false}},
			{ID: "b", Name: "B", IndexingStrategy: types.IndexingStrategy{WikiEnabled: true}},
		}}),
		wiki: &fakeWiki{graph: &types.WikiGraphData{Meta: types.WikiGraphMeta{Returned: 1}}},
	}
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{}
	res, err := svc.handleWikiGraph(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %s", toolText(t, res))
	}
	text := toolText(t, res)
	var payload struct {
		Graphs []struct {
			KBID string `json:"kb_id"`
		} `json:"graphs"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Graphs) != 1 || payload.Graphs[0].KBID != "b" {
		t.Fatalf("got %#v", payload.Graphs)
	}
}

func TestNewHTTPHandler_ToolsList(t *testing.T) {
	h := NewHTTPHandler(Dependencies{
		KBService:   nil, // tools registered regardless; handler construction only needs services for calls
		WikiService: nil,
	})
	// NewHTTPHandler with nil services still builds; GinHandler would NPE on listAuthorized.
	// Rebuild with listOnly via custom register — smoke: handler non-nil.
	if h == nil {
		t.Fatal("handler nil")
	}
}

func toolText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if len(res.Content) == 0 {
		t.Fatal("no content")
	}
	tc, ok := res.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("content type %T", res.Content[0])
	}
	return tc.Text
}
