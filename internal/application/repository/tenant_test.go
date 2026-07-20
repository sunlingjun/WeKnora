package repository

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestDB creates an in-memory SQLite database with tenant table.
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.Tenant{}, &types.TenantMember{}, &types.User{}))
	return db
}

func TestDeleteTenant_SoftDeletesMemberships(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	repo := NewTenantRepository(db)

	tenant := &types.Tenant{Name: "gone", Status: "active"}
	require.NoError(t, db.Create(tenant).Error)

	member := &types.TenantMember{
		UserID:   "user-1",
		TenantID: tenant.ID,
		Role:     types.TenantRoleOwner,
		Status:   types.TenantMemberStatusActive,
	}
	require.NoError(t, db.Create(member).Error)

	require.NoError(t, repo.DeleteTenant(ctx, tenant.ID))

	var tenantCount int64
	require.NoError(t, db.Model(&types.Tenant{}).Count(&tenantCount).Error)
	assert.Equal(t, int64(0), tenantCount)

	var memberCount int64
	require.NoError(t, db.Model(&types.TenantMember{}).Count(&memberCount).Error)
	assert.Equal(t, int64(0), memberCount)

	// Unscoped: rows still exist but are soft-deleted.
	var rawTenantCount int64
	require.NoError(t, db.Unscoped().Model(&types.Tenant{}).Count(&rawTenantCount).Error)
	assert.Equal(t, int64(1), rawTenantCount)

	var rawMemberCount int64
	require.NoError(t, db.Unscoped().Model(&types.TenantMember{}).Count(&rawMemberCount).Error)
	assert.Equal(t, int64(1), rawMemberCount)
}

func TestDeleteTenant_RebindsHomeUserToSurvivingOwnerTenant(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	repo := NewTenantRepository(db)

	keep := &types.Tenant{Name: "keep", Status: "active", StorageUsed: 100}
	gone := &types.Tenant{Name: "gone", Status: "active", StorageUsed: 0}
	require.NoError(t, db.Create(keep).Error)
	require.NoError(t, db.Create(gone).Error)

	lastActive := gone.ID
	user := &types.User{
		ID:           "user-1",
		Username:     "u1",
		Email:        "u1@example.com",
		PasswordHash: "x",
		TenantID:     gone.ID,
		Preferences: types.UserPreferences{
			LastActiveTenantID: &lastActive,
		},
	}
	require.NoError(t, db.Create(user).Error)

	require.NoError(t, db.Create(&types.TenantMember{
		UserID: user.ID, TenantID: keep.ID,
		Role: types.TenantRoleOwner, Status: types.TenantMemberStatusActive,
	}).Error)
	require.NoError(t, db.Create(&types.TenantMember{
		UserID: user.ID, TenantID: gone.ID,
		Role: types.TenantRoleOwner, Status: types.TenantMemberStatusActive,
	}).Error)

	require.NoError(t, repo.DeleteTenant(ctx, gone.ID))

	var got types.User
	require.NoError(t, db.First(&got, "id = ?", user.ID).Error)
	assert.Equal(t, keep.ID, got.TenantID)
	require.NotNil(t, got.Preferences.LastActiveTenantID)
	assert.Equal(t, keep.ID, *got.Preferences.LastActiveTenantID)
}

func TestDeleteTenant_NoSurvivingMembership_SetsHomeZero(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	repo := NewTenantRepository(db)

	gone := &types.Tenant{Name: "gone", Status: "active"}
	require.NoError(t, db.Create(gone).Error)

	user := &types.User{
		ID: "user-1", Username: "u1", Email: "u1@example.com", PasswordHash: "x", TenantID: gone.ID,
	}
	require.NoError(t, db.Create(user).Error)
	require.NoError(t, db.Create(&types.TenantMember{
		UserID: user.ID, TenantID: gone.ID,
		Role: types.TenantRoleOwner, Status: types.TenantMemberStatusActive,
	}).Error)

	require.NoError(t, repo.DeleteTenant(ctx, gone.ID))

	var got types.User
	require.NoError(t, db.First(&got, "id = ?", user.ID).Error)
	assert.Equal(t, uint64(0), got.TenantID)
}
