package chatpipeline

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

type openRetrieveTenantService struct {
	interfaces.TenantService
}

func (s *openRetrieveTenantService) GetTenantByID(_ context.Context, id uint64) (*types.Tenant, error) {
	return &types.Tenant{ID: id}, nil
}

type openRetrieveSearchService struct {
	interfaces.KnowledgeBaseService
	mu    sync.Mutex
	calls []openRetrieveSearchCall
	fail  map[string]error
}

type openRetrieveSearchCall struct {
	tenant uint64
	info   uint64
	kbIDs  []string
}

func (s *openRetrieveSearchService) GetKnowledgeBasesByIDsOnly(
	_ context.Context, ids []string,
) ([]*types.KnowledgeBase, error) {
	kbs := make([]*types.KnowledgeBase, 0, len(ids))
	for _, id := range ids {
		tenantID := uint64(11)
		if id == "kb-b" {
			tenantID = 22
		}
		kbs = append(kbs, &types.KnowledgeBase{
			ID:               id,
			TenantID:         tenantID,
			Type:             types.KnowledgeBaseTypeDocument,
			EmbeddingModelID: "embed-shared",
			IndexingStrategy: types.DefaultIndexingStrategy(),
		})
	}
	return kbs, nil
}

func (s *openRetrieveSearchService) ResolveEmbeddingModelKeys(
	_ context.Context, kbs []*types.KnowledgeBase,
) map[string]string {
	keys := make(map[string]string, len(kbs))
	for _, kb := range kbs {
		keys[kb.ID] = "shared-model|https://embed.example"
	}
	return keys
}

func (s *openRetrieveSearchService) GetQueryEmbedding(context.Context, string, string) ([]float32, error) {
	return []float32{0.1, 0.2}, nil
}

func (s *openRetrieveSearchService) HybridSearch(
	ctx context.Context, id string, params types.SearchParams,
) ([]*types.SearchResult, error) {
	tenantID := types.MustTenantIDFromContext(ctx)
	info, _ := types.TenantInfoFromContext(ctx)
	var infoID uint64
	if info != nil {
		infoID = info.ID
	}
	s.mu.Lock()
	s.calls = append(s.calls, openRetrieveSearchCall{
		tenant: tenantID,
		info:   infoID,
		kbIDs:  append([]string(nil), params.KnowledgeBaseIDs...),
	})
	s.mu.Unlock()
	if err := s.fail[id]; err != nil {
		return nil, err
	}
	return []*types.SearchResult{{
		ID:              id + "-chunk",
		KnowledgeBaseID: id,
		Content:         id,
	}}, nil
}

func openRetrieveChat(ctx context.Context) (*types.ChatManage, context.Context, *types.OpenRetrieveWarningCollector) {
	ctx = types.WithOpenRetrieve(ctx)
	ctx = context.WithValue(ctx, types.TenantIDContextKey, uint64(11))
	ctx = context.WithValue(ctx, types.TenantInfoContextKey, &types.Tenant{ID: 11})
	ctx, box := types.WithOpenRetrieveWarningCollector(ctx)
	chat := &types.ChatManage{
		PipelineRequest: types.PipelineRequest{
			SearchTargets: types.SearchTargets{
				{Type: types.SearchTargetTypeKnowledgeBase, KnowledgeBaseID: "kb-a", TenantID: 11},
				{Type: types.SearchTargetTypeKnowledgeBase, KnowledgeBaseID: "kb-b", TenantID: 22},
			},
			EmbeddingTopK: 8,
		},
		PipelineState: types.PipelineState{RewriteQuery: "员工请假制度"},
	}
	return chat, ctx, box
}

func TestOpenRetrieveSearchesEachOwnerTenant(t *testing.T) {
	svc := &openRetrieveSearchService{}
	plugin := &PluginSearch{
		knowledgeBaseService: svc,
		tenantService:        &openRetrieveTenantService{},
	}
	chat, ctx, box := openRetrieveChat(context.Background())

	err := plugin.OnEvent(ctx, types.CHUNK_SEARCH, chat, func() *PluginError { return nil })
	if err != nil {
		t.Fatalf("search returned %v", err)
	}
	if len(chat.SearchResult) != 2 {
		t.Fatalf("hits=%d, want both tenants", len(chat.SearchResult))
	}
	if len(box.Items()) != 0 {
		t.Fatalf("unexpected warnings: %+v", box.Items())
	}
	seen := map[uint64][]string{}
	for _, call := range svc.calls {
		if call.tenant != call.info {
			t.Fatalf("execution tenant %d != tenant info %d", call.tenant, call.info)
		}
		seen[call.tenant] = call.kbIDs
	}
	if len(seen[11]) != 1 || seen[11][0] != "kb-a" {
		t.Fatalf("tenant 11 search = %v", seen[11])
	}
	if len(seen[22]) != 1 || seen[22][0] != "kb-b" {
		t.Fatalf("tenant 22 search = %v", seen[22])
	}
}

func TestOpenRetrieveKeepsOtherTenantWhenOneSearchFails(t *testing.T) {
	svc := &openRetrieveSearchService{fail: map[string]error{
		"kb-b": errors.New("store unavailable"),
	}}
	plugin := &PluginSearch{
		knowledgeBaseService: svc,
		tenantService:        &openRetrieveTenantService{},
	}
	chat, ctx, box := openRetrieveChat(context.Background())

	err := plugin.OnEvent(ctx, types.CHUNK_SEARCH, chat, func() *PluginError { return nil })
	if err != nil {
		t.Fatalf("partial failure became %v", err)
	}
	if len(chat.SearchResult) != 1 || chat.SearchResult[0].KnowledgeBaseID != "kb-a" {
		t.Fatalf("results = %+v", chat.SearchResult)
	}
	warnings := box.Items()
	if len(warnings) != 1 {
		t.Fatalf("warnings = %+v", warnings)
	}
	if len(warnings[0].KnowledgeBaseIDs) != 1 || warnings[0].KnowledgeBaseIDs[0] != "kb-b" {
		t.Fatalf("warning kbs = %v", warnings[0].KnowledgeBaseIDs)
	}
	if warnings[0].Reason == "" {
		t.Fatal("warning reason is empty")
	}
}
