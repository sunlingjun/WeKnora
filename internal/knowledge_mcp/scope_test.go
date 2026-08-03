package knowledge_mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
)

func TestResolveEffective_UnrestrictedUsesAll(t *testing.T) {
	r := newScopeResolver(&listOnlyKB{list: []*types.KnowledgeBase{
		{ID: "a", Name: "A"},
		{ID: "b", Name: "B"},
	}})
	ctx := context.Background()
	got, err := r.resolveEffective(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d, want 2", len(got))
	}
}

func TestResolveEffective_RestrictedFilters(t *testing.T) {
	r := newScopeResolver(&listOnlyKB{list: []*types.KnowledgeBase{
		{ID: "a", Name: "A"},
		{ID: "b", Name: "B"},
		{ID: "c", Name: "C"},
	}})
	ctx := types.WithTenantAPIKeyScope(context.Background(), types.TenantAPIKeyScope{
		KnowledgeBaseIDs: types.StringArray{"a", "b"},
	})
	got, err := r.resolveEffective(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d want 2", len(got))
	}
}

func TestResolveEffective_Intersection(t *testing.T) {
	r := newScopeResolver(&listOnlyKB{list: []*types.KnowledgeBase{
		{ID: "a", Name: "A"},
		{ID: "b", Name: "B"},
	}})
	ctx := types.WithTenantAPIKeyScope(context.Background(), types.TenantAPIKeyScope{
		KnowledgeBaseIDs: types.StringArray{"a", "b"},
	})
	got, err := r.resolveEffective(ctx, []string{"a"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "a" {
		t.Fatalf("got %#v", got)
	}
}

func TestResolveEffective_OutOfScopeStrict(t *testing.T) {
	r := newScopeResolver(&listOnlyKB{list: []*types.KnowledgeBase{
		{ID: "a", Name: "A"},
	}})
	ctx := types.WithTenantAPIKeyScope(context.Background(), types.TenantAPIKeyScope{
		KnowledgeBaseIDs: types.StringArray{"a"},
	})
	_, err := r.resolveEffective(ctx, []string{"a", "evil"})
	if err == nil {
		t.Fatal("expected error")
	}
	se, ok := err.(*ScopeError)
	if !ok || se.Code != codeKBOutOfScope {
		t.Fatalf("got %#v", err)
	}
	if !strings.Contains(se.Message, "evil") {
		t.Fatalf("message = %s", se.Message)
	}
}

func TestResolveEffective_NoKBInScope(t *testing.T) {
	r := newScopeResolver(&listOnlyKB{list: nil})
	_, err := r.resolveEffective(context.Background(), nil)
	se, ok := err.(*ScopeError)
	if !ok || se.Code != codeNoKBInScope {
		t.Fatalf("got %#v", err)
	}
}

func TestFilterWikiEnabled(t *testing.T) {
	wikiOn := &types.KnowledgeBase{ID: "w", IndexingStrategy: types.IndexingStrategy{WikiEnabled: true}}
	wikiOff := &types.KnowledgeBase{ID: "d", IndexingStrategy: types.IndexingStrategy{WikiEnabled: false}}
	got := filterWikiEnabled([]*types.KnowledgeBase{wikiOn, wikiOff})
	if len(got) != 1 || got[0].ID != "w" {
		t.Fatalf("got %#v", got)
	}
}

func TestApplyLimit(t *testing.T) {
	applied, requested, err := applyLimit(0, false, 10, 50)
	if err != nil || applied != 10 || requested != 10 {
		t.Fatalf("default: %d %d %v", applied, requested, err)
	}
	applied, requested, err = applyLimit(100, true, 10, 50)
	if err != nil || applied != 50 || requested != 100 {
		t.Fatalf("clamp: %d %d %v", applied, requested, err)
	}
	_, _, err = applyLimit(0, true, 10, 50)
	if err == nil || err.Code != codeInvalidLimit {
		t.Fatalf("want invalid_limit, got %#v", err)
	}
}

// listOnlyKB only implements ListKnowledgeBases for scope tests.
type listOnlyKB struct {
	list []*types.KnowledgeBase
	err  error
}

func (l *listOnlyKB) ListKnowledgeBases(context.Context) ([]*types.KnowledgeBase, error) {
	return l.list, l.err
}
