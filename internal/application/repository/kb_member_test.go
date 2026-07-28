package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupKBMemberTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.KnowledgeBaseMember{}))
	// Simulate the legacy unique constraint that causes leave→rejoin→leave failures.
	require.NoError(t, db.Exec(`
		CREATE UNIQUE INDEX uk_kb_members_kb_user
		ON knowledge_base_members (knowledge_base_id, user_id, deleted_at)
	`).Error)
	return db
}

func TestDeleteMember_LeaveRejoinLeave_DoesNotHitUniqueConstraint(t *testing.T) {
	db := setupKBMemberTestDB(t)
	repo := NewKnowledgeBaseMemberRepository(db)
	ctx := context.Background()

	kbID, userID := "kb-1", "user-1"
	member := &types.KnowledgeBaseMember{
		ID:              "m-1",
		KnowledgeBaseID: kbID,
		UserID:          userID,
		TenantID:        1,
		Role:            types.KBMemberRoleViewer,
		JoinedAt:        time.Now(),
	}
	require.NoError(t, repo.CreateMember(ctx, member))

	// First leave
	require.NoError(t, repo.DeleteMember(ctx, kbID, userID))
	_, err := repo.GetMemberByKBAndUser(ctx, kbID, userID)
	require.ErrorIs(t, err, ErrKnowledgeBaseMemberNotFound)

	// Rejoin via restore path
	soft, err := repo.GetSoftDeletedMemberByKBAndUser(ctx, kbID, userID)
	require.NoError(t, err)
	soft.Role = types.KBMemberRoleViewer
	soft.TenantID = 1
	require.NoError(t, repo.RestoreMember(ctx, soft))

	active, err := repo.GetMemberByKBAndUser(ctx, kbID, userID)
	require.NoError(t, err)
	assert.Equal(t, "m-1", active.ID)

	// Second leave must succeed even under the legacy unique(kb,user,deleted_at)
	require.NoError(t, repo.DeleteMember(ctx, kbID, userID))

	var raw int64
	require.NoError(t, db.Unscoped().Model(&types.KnowledgeBaseMember{}).
		Where("knowledge_base_id = ? AND user_id = ?", kbID, userID).
		Count(&raw).Error)
	assert.Equal(t, int64(1), raw, "exactly one soft-deleted row should remain")
}

func TestDeleteMember_CleansPriorSoftDeletedRows(t *testing.T) {
	db := setupKBMemberTestDB(t)
	repo := NewKnowledgeBaseMemberRepository(db)
	ctx := context.Background()

	kbID, userID := "kb-2", "user-2"
	ts := time.Now().Add(-time.Hour)
	require.NoError(t, db.Create(&types.KnowledgeBaseMember{
		ID: "old-soft", KnowledgeBaseID: kbID, UserID: userID, TenantID: 1,
		Role: types.KBMemberRoleViewer, JoinedAt: ts,
		DeletedAt: gorm.DeletedAt{Time: ts, Valid: true},
	}).Error)
	require.NoError(t, repo.CreateMember(ctx, &types.KnowledgeBaseMember{
		ID: "active", KnowledgeBaseID: kbID, UserID: userID, TenantID: 1,
		Role: types.KBMemberRoleViewer, JoinedAt: time.Now(),
	}))

	require.NoError(t, repo.DeleteMember(ctx, kbID, userID))

	var ids []string
	require.NoError(t, db.Unscoped().Model(&types.KnowledgeBaseMember{}).
		Where("knowledge_base_id = ? AND user_id = ?", kbID, userID).
		Pluck("id", &ids).Error)
	assert.Equal(t, []string{"active"}, ids)
}
