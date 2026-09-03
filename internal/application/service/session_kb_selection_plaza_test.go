package service

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

type listKBStub struct {
	interfaces.KnowledgeBaseService
	kbs []*types.KnowledgeBase
}

func (s *listKBStub) ListKnowledgeBases(context.Context) ([]*types.KnowledgeBase, error) {
	return s.kbs, nil
}

type joinedPlazaListStub struct {
	interfaces.SharedKnowledgeBaseService
	kbs []*types.KnowledgeBase
}

func (s *joinedPlazaListStub) ListJoinedSharedKnowledgeBases(context.Context) ([]*types.KnowledgeBase, error) {
	return s.kbs, nil
}

func TestResolveKnowledgeBasesFromAgent_IncludesPlazaJoined(t *testing.T) {
	svc := &sessionService{
		knowledgeBaseService: &listKBStub{kbs: []*types.KnowledgeBase{
			{ID: "kb-own", TenantID: 10035, Name: "猪病知识库"},
		}},
		sharedKBService: &joinedPlazaListStub{kbs: []*types.KnowledgeBase{
			{ID: "kb-plaza", TenantID: 10038, Name: "学联网", Visibility: types.KnowledgeBaseVisibilityShared},
		}},
	}
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(10035))
	ctx = context.WithValue(ctx, types.UserIDContextKey, "user-1")

	got := svc.resolveKnowledgeBasesFromAgent(ctx, &types.CustomAgent{
		TenantID: 10035,
		Config:   types.CustomAgentConfig{KBSelectionMode: "all"},
	}, 10035)

	require.Equal(t, []string{"kb-own", "kb-plaza"}, got)
}
