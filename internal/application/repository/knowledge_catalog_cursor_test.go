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

func TestListKnowledgeCatalogCursor_OwnerTenantAndStableOrder(t *testing.T) {
	dsn := "file:" + uuid.New().String() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&types.Knowledge{}))

	repo := NewKnowledgeRepository(db)
	now := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	rows := []*types.Knowledge{
		{ID: "k-caller", TenantID: 42, KnowledgeBaseID: "kb-share-9", Type: "file", ParseStatus: types.ParseStatusCompleted, UpdatedAt: now},
		{ID: "k-b", TenantID: 7, KnowledgeBaseID: "kb-share-9", Type: "file", ParseStatus: types.ParseStatusCompleted, UpdatedAt: now.Add(time.Minute)},
		{ID: "k-a", TenantID: 7, KnowledgeBaseID: "kb-share-9", Type: "file", ParseStatus: types.ParseStatusCompleted, UpdatedAt: now},
		{ID: "k-del", TenantID: 7, KnowledgeBaseID: "kb-share-9", Type: "file", ParseStatus: types.ParseStatusDeleting, UpdatedAt: now.Add(2 * time.Minute)},
		{ID: "k-other", TenantID: 7, KnowledgeBaseID: "kb-other", Type: "file", ParseStatus: types.ParseStatusCompleted, UpdatedAt: now},
	}
	for _, row := range rows {
		require.NoError(t, db.Create(row).Error)
	}

	first, err := repo.ListKnowledgeCatalogCursor(context.Background(), 7, "kb-share-9", types.KnowledgeCatalogCursorQuery{Limit: 1})
	require.NoError(t, err)
	require.Len(t, first, 1)
	require.Equal(t, "k-a", first[0].ID)

	second, err := repo.ListKnowledgeCatalogCursor(context.Background(), 7, "kb-share-9", types.KnowledgeCatalogCursorQuery{
		Limit:           10,
		HasCursor:       true,
		CursorUpdatedAt: first[0].UpdatedAt,
		CursorID:        first[0].ID,
	})
	require.NoError(t, err)
	require.Len(t, second, 1)
	require.Equal(t, "k-b", second[0].ID)

	wrongTenant, err := repo.ListKnowledgeCatalogCursor(context.Background(), 42, "kb-share-9", types.KnowledgeCatalogCursorQuery{Limit: 10})
	require.NoError(t, err)
	require.Len(t, wrongTenant, 1)
	require.Equal(t, "k-caller", wrongTenant[0].ID)
}
