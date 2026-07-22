package service

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

type stubJoinedMemberRepo struct {
	interfaces.KnowledgeBaseMemberRepository
	members []*types.KnowledgeBaseMember
}

func (r *stubJoinedMemberRepo) ListMembersByUser(context.Context, string) ([]*types.KnowledgeBaseMember, error) {
	return r.members, nil
}

type stubJoinedKBRepo struct {
	interfaces.KnowledgeBaseRepository
	byID map[string]*types.KnowledgeBase
}

func (r *stubJoinedKBRepo) GetKnowledgeBaseByIDs(_ context.Context, ids []string) ([]*types.KnowledgeBase, error) {
	out := make([]*types.KnowledgeBase, 0, len(ids))
	for _, id := range ids {
		if kb, ok := r.byID[id]; ok {
			out = append(out, kb)
		}
	}
	return out, nil
}

func TestListJoinedSharedKnowledgeBases_FiltersSameTenantAndOwner(t *testing.T) {
	const (
		userID        = "user-a"
		currentTenant = uint64(100)
		otherTenant   = uint64(200)
	)

	svc := &sharedKnowledgeBaseService{
		kbRepo: &stubJoinedKBRepo{byID: map[string]*types.KnowledgeBase{
			"same-tenant": {
				ID: "same-tenant", TenantID: currentTenant,
				Visibility: types.KnowledgeBaseVisibilityShared, OwnerID: "owner-b",
			},
			"cross-tenant": {
				ID: "cross-tenant", TenantID: otherTenant,
				Visibility: types.KnowledgeBaseVisibilityShared, OwnerID: "owner-c",
			},
			"own-shared": {
				ID: "own-shared", TenantID: otherTenant,
				Visibility: types.KnowledgeBaseVisibilityShared, OwnerID: userID,
			},
			"private": {
				ID: "private", TenantID: otherTenant,
				Visibility: types.KnowledgeBaseVisibilityPrivate, OwnerID: "owner-c",
			},
		}},
		memberRepo: &stubJoinedMemberRepo{members: []*types.KnowledgeBaseMember{
			{KnowledgeBaseID: "same-tenant"},
			{KnowledgeBaseID: "cross-tenant"},
			{KnowledgeBaseID: "own-shared"},
			{KnowledgeBaseID: "private"},
		}},
	}

	ctx := context.WithValue(context.Background(), types.UserIDContextKey, userID)
	ctx = context.WithValue(ctx, types.TenantIDContextKey, currentTenant)

	got, err := svc.ListJoinedSharedKnowledgeBases(ctx)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "cross-tenant", got[0].ID)
}
