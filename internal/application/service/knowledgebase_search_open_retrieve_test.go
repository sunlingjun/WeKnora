package service

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

type openRetrieveKnowledgeRepo struct {
	interfaces.KnowledgeRepository
	rows []*types.Knowledge
}

func (r *openRetrieveKnowledgeRepo) GetKnowledgeBatch(_ context.Context, tenantID uint64, ids []string) ([]*types.Knowledge, error) {
	return filterKnowledge(r.rows, func(k *types.Knowledge) bool {
		return k.TenantID == tenantID && containsID(ids, k.ID)
	}), nil
}

func (r *openRetrieveKnowledgeRepo) GetKnowledgeBatchByIDOnly(_ context.Context, ids []string) ([]*types.Knowledge, error) {
	return filterKnowledge(r.rows, func(k *types.Knowledge) bool {
		return containsID(ids, k.ID)
	}), nil
}

type openRetrieveChunkRepo struct {
	interfaces.ChunkRepository
	rows []*types.Chunk
}

func (r *openRetrieveChunkRepo) ListChunksByID(_ context.Context, tenantID uint64, ids []string) ([]*types.Chunk, error) {
	return filterChunks(r.rows, func(c *types.Chunk) bool {
		return c.TenantID == tenantID && containsID(ids, c.ID)
	}), nil
}

func (r *openRetrieveChunkRepo) ListChunksByIDOnly(_ context.Context, ids []string) ([]*types.Chunk, error) {
	return filterChunks(r.rows, func(c *types.Chunk) bool {
		return containsID(ids, c.ID)
	}), nil
}

func (r *openRetrieveChunkRepo) ListChunksByParentIDsOnly(context.Context, []string) ([]*types.Chunk, error) {
	return nil, nil
}

func filterKnowledge(rows []*types.Knowledge, keep func(*types.Knowledge) bool) []*types.Knowledge {
	var out []*types.Knowledge
	for _, row := range rows {
		if keep(row) {
			out = append(out, row)
		}
	}
	return out
}

func filterChunks(rows []*types.Chunk, keep func(*types.Chunk) bool) []*types.Chunk {
	var out []*types.Chunk
	for _, row := range rows {
		if keep(row) {
			out = append(out, row)
		}
	}
	return out
}

func containsID(ids []string, id string) bool {
	for _, candidate := range ids {
		if candidate == id {
			return true
		}
	}
	return false
}

func openRetrieveFixture() (*knowledgeBaseService, []*types.IndexWithScore) {
	knowledges := []*types.Knowledge{
		{ID: "doc-a", TenantID: 11, KnowledgeBaseID: "kb-a", Title: "猪病"},
		{ID: "doc-b", TenantID: 22, KnowledgeBaseID: "kb-b", Title: "员工手册"},
	}
	chunks := []*types.Chunk{
		{ID: "chunk-a", TenantID: 11, KnowledgeID: "doc-a", KnowledgeBaseID: "kb-a", Content: "猪口蹄疫", ChunkType: types.ChunkTypeText, IsEnabled: true},
		{ID: "chunk-b", TenantID: 22, KnowledgeID: "doc-b", KnowledgeBaseID: "kb-b", Content: "员工请假制度", ChunkType: types.ChunkTypeText, IsEnabled: true},
	}
	svc := &knowledgeBaseService{
		kgRepo:    &openRetrieveKnowledgeRepo{rows: knowledges},
		chunkRepo: &openRetrieveChunkRepo{rows: chunks},
	}
	hits := []*types.IndexWithScore{
		{ChunkID: "chunk-a", KnowledgeID: "doc-a", KnowledgeBaseID: "kb-a", Score: 0.9, MatchType: types.MatchTypeEmbedding},
		{ChunkID: "chunk-b", KnowledgeID: "doc-b", KnowledgeBaseID: "kb-b", Score: 0.8, MatchType: types.MatchTypeEmbedding},
	}
	return svc, hits
}

func TestOpenRetrieveHydratesChunksFromEveryTenant(t *testing.T) {
	svc, hits := openRetrieveFixture()
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(11))
	ctx = types.WithOpenRetrieve(ctx)

	got, err := svc.processSearchResults(ctx, hits, true)
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, "kb-a", got[0].KnowledgeBaseID)
	require.Equal(t, "kb-b", got[1].KnowledgeBaseID)
	require.Equal(t, "员工手册", got[1].KnowledgeTitle)
}

func TestLoggedInSearchDoesNotHydrateForeignChunksWithoutAUser(t *testing.T) {
	svc, hits := openRetrieveFixture()
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(11))

	got, err := svc.processSearchResults(ctx, hits, true)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "kb-a", got[0].KnowledgeBaseID)
}
