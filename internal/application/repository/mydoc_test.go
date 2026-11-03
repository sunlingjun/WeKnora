package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openMyDocTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.MyDocNode{}, &types.MyDocFile{}, &types.MyDocShare{}, &types.MyDocTicket{}))
	return db
}

func TestMyDocRepo_CreateFolder_DuplicateName(t *testing.T) {
	repo := NewMyDocRepository(openMyDocTestDB(t))
	ctx := context.Background()
	owner := "user-a"
	n1 := &types.MyDocNode{
		ID: uuid.NewString(), OwnerUserID: owner, Name: "Docs", Type: types.MyDocNodeFolder,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	require.NoError(t, repo.CreateNode(ctx, n1))
	dup, err := repo.FindSiblingByName(ctx, owner, nil, "Docs", "")
	require.NoError(t, err)
	require.NotNil(t, dup)
	require.Equal(t, n1.ID, dup.ID)
}

func TestMyDocRepo_Move_IsAncestor(t *testing.T) {
	repo := NewMyDocRepository(openMyDocTestDB(t))
	ctx := context.Background()
	owner := "user-a"
	a := &types.MyDocNode{ID: uuid.NewString(), OwnerUserID: owner, Name: "A", Type: types.MyDocNodeFolder, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	require.NoError(t, repo.CreateNode(ctx, a))
	bParent := a.ID
	b := &types.MyDocNode{ID: uuid.NewString(), OwnerUserID: owner, ParentID: &bParent, Name: "B", Type: types.MyDocNodeFolder, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	require.NoError(t, repo.CreateNode(ctx, b))
	cParent := b.ID
	c := &types.MyDocNode{ID: uuid.NewString(), OwnerUserID: owner, ParentID: &cParent, Name: "C", Type: types.MyDocNodeFolder, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	require.NoError(t, repo.CreateNode(ctx, c))

	ok, err := repo.IsAncestor(ctx, a.ID, c.ID)
	require.NoError(t, err)
	require.True(t, ok)
	// Moving A under C would cycle because A is ancestor of C
	cycle, err := repo.IsAncestor(ctx, a.ID, c.ID)
	require.NoError(t, err)
	require.True(t, cycle)
}

func TestMyDocRepo_ListOwned_ExcludesArchived(t *testing.T) {
	repo := NewMyDocRepository(openMyDocTestDB(t))
	ctx := context.Background()
	owner := "user-a"
	alive := &types.MyDocNode{ID: uuid.NewString(), OwnerUserID: owner, Name: "alive", Type: types.MyDocNodeFolder, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	require.NoError(t, repo.CreateNode(ctx, alive))
	gone := &types.MyDocNode{ID: uuid.NewString(), OwnerUserID: owner, Name: "gone", Type: types.MyDocNodeFolder, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	require.NoError(t, repo.CreateNode(ctx, gone))
	require.NoError(t, repo.ArchiveSubtree(ctx, owner, gone.ID))
	nodes, err := repo.ListOwned(ctx, owner, false)
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.Equal(t, alive.ID, nodes[0].ID)
}
