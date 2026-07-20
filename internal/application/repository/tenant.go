package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrTenantNotFound         = errors.New("tenant not found")
	ErrTenantHasKnowledgeBase = errors.New("tenant has associated knowledge bases")
)

// tenantRepository implements tenant repository interface
type tenantRepository struct {
	db *gorm.DB
}

// NewTenantRepository creates a new tenant repository
func NewTenantRepository(db *gorm.DB) interfaces.TenantRepository {
	return &tenantRepository{db: db}
}

// CreateTenant creates tenant
func (r *tenantRepository) CreateTenant(ctx context.Context, tenant *types.Tenant) error {
	return r.db.WithContext(ctx).Create(tenant).Error
}

// GetTenantByID gets tenant by ID
func (r *tenantRepository) GetTenantByID(ctx context.Context, id uint64) (*types.Tenant, error) {
	var tenant types.Tenant
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&tenant).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTenantNotFound
		}
		return nil, err
	}
	return &tenant, nil
}

// GetTenantsByIDs batches GetTenantByID with a single IN-list query.
// Returns a map keyed by tenant ID; missing rows are simply absent from
// the map (no error). An empty input slice short-circuits to an empty map
// without hitting the database.
func (r *tenantRepository) GetTenantsByIDs(ctx context.Context, ids []uint64) (map[uint64]*types.Tenant, error) {
	if len(ids) == 0 {
		return map[uint64]*types.Tenant{}, nil
	}
	var tenants []*types.Tenant
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&tenants).Error; err != nil {
		return nil, err
	}
	out := make(map[uint64]*types.Tenant, len(tenants))
	for _, t := range tenants {
		if t != nil {
			out[t.ID] = t
		}
	}
	return out, nil
}

// ListTenants lists all tenants
func (r *tenantRepository) ListTenants(ctx context.Context) ([]*types.Tenant, error) {
	var tenants []*types.Tenant
	if err := r.db.WithContext(ctx).Order("created_at DESC").Find(&tenants).Error; err != nil {
		return nil, err
	}
	return tenants, nil
}

// SearchTenants searches tenants with pagination and filters
func (r *tenantRepository) SearchTenants(ctx context.Context, keyword string, tenantID uint64, page, pageSize int) ([]*types.Tenant, int64, error) {
	var tenants []*types.Tenant
	var total int64

	query := r.db.WithContext(ctx).Model(&types.Tenant{})

	// Build search conditions
	if tenantID > 0 && keyword != "" {
		escaped := escapeLikeKeyword(keyword)
		query = query.Where("id = ? OR name LIKE ? OR description LIKE ?", tenantID, "%"+escaped+"%", "%"+escaped+"%")
	} else if tenantID > 0 {
		query = query.Where("id = ?", tenantID)
	} else if keyword != "" {
		escaped := escapeLikeKeyword(keyword)
		query = query.Where("name LIKE ? OR description LIKE ?", "%"+escaped+"%", "%"+escaped+"%")
	}

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination
	if page > 0 && pageSize > 0 {
		offset := (page - 1) * pageSize
		query = query.Offset(offset).Limit(pageSize)
	}

	// Order by created_at DESC
	query = query.Order("created_at DESC")

	// Execute query
	if err := query.Find(&tenants).Error; err != nil {
		return nil, 0, err
	}

	return tenants, total, nil
}

// UpdateTenant updates tenant.
func (r *tenantRepository) UpdateTenant(ctx context.Context, tenant *types.Tenant) error {
	return r.db.WithContext(ctx).Model(&types.Tenant{}).Where("id = ?", tenant.ID).Updates(tenant).Error
}

// DeleteTenant soft-deletes the tenant and every active membership row
// for that tenant, then rebinds any user whose home (users.tenant_id)
// pointed at the deleted tenant.
//
// Rebind rule (same as CAS AutoBindTenant recovery): prefer an active
// owner/admin membership on a still-living tenant with storage_used>0,
// else the oldest such tenant; if none remain, set home to 0 so the next
// CAS login creates a fresh default workspace instead of looping on a
// zombie pointer.
func (r *tenantRepository) DeleteTenant(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var homeUsers []types.User
		if err := tx.Where("tenant_id = ?", id).Find(&homeUsers).Error; err != nil {
			return err
		}

		if err := tx.Where("tenant_id = ?", id).Delete(&types.TenantMember{}).Error; err != nil {
			return err
		}
		if err := tx.Where("id = ?", id).Delete(&types.Tenant{}).Error; err != nil {
			return err
		}

		for i := range homeUsers {
			u := &homeUsers[i]
			keepID, err := pickHomeTenantAfterDelete(tx, u.ID)
			if err != nil {
				return err
			}
			u.TenantID = keepID
			u.UpdatedAt = time.Now()
			if u.Preferences.LastActiveTenantID != nil && *u.Preferences.LastActiveTenantID == id {
				if keepID > 0 {
					v := keepID
					u.Preferences.LastActiveTenantID = &v
				} else {
					u.Preferences.LastActiveTenantID = nil
				}
			}
			if err := tx.Model(u).Select("TenantID", "Preferences", "UpdatedAt").Updates(u).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// pickHomeTenantAfterDelete selects a replacement home for userID after
// the deleted tenant's memberships are already soft-deleted.
func pickHomeTenantAfterDelete(tx *gorm.DB, userID string) (uint64, error) {
	type row struct {
		TenantID    uint64
		StorageUsed int64
		CreatedAt   time.Time
	}
	var rows []row
	err := tx.Table("tenant_members AS tm").
		Select("tm.tenant_id AS tenant_id, t.storage_used AS storage_used, t.created_at AS created_at").
		Joins("JOIN tenants AS t ON t.id = tm.tenant_id AND t.deleted_at IS NULL").
		Where(
			"tm.user_id = ? AND tm.deleted_at IS NULL AND tm.status = ? AND tm.role IN ?",
			userID,
			types.TenantMemberStatusActive,
			[]string{string(types.TenantRoleOwner), string(types.TenantRoleAdmin)},
		).
		Order("CASE WHEN t.storage_used > 0 THEN 0 ELSE 1 END ASC, t.created_at ASC, t.id ASC").
		Scan(&rows).Error
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return rows[0].TenantID, nil
}

func (r *tenantRepository) AdjustStorageUsed(ctx context.Context, tenantID uint64, delta int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var tenant types.Tenant
		// 使用悲观锁确保并发安全
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&tenant, tenantID).Error; err != nil {
			return err
		}

		tenant.StorageUsed += delta
		// 保存更新并验证业务规则
		if tenant.StorageUsed < 0 {
			logger.Errorf(ctx, "tenant storage used is negative %d: %d", tenant.ID, tenant.StorageUsed)
			tenant.StorageUsed = 0
		}

		return tx.Save(&tenant).Error
	})
}

// BulkSetStorageQuota writes quotaBytes to storage_quota for every
// tenant in one statement. We don't WHERE-filter (the action is
// "apply globally"), so the affected count equals the row count of
// the tenants table.
//
// No transaction here: the operation is a single statement and we
// don't want to hold a long lock just to update a single column. If
// a concurrent CreateTenant lands in the middle, the new row gets
// the new default via the system-setting resolver in the handler —
// no risk of the new tenant being skipped.
func (r *tenantRepository) BulkSetStorageQuota(ctx context.Context, quotaBytes int64) (int64, error) {
	res := r.db.WithContext(ctx).
		Model(&types.Tenant{}).
		Where("1 = 1"). // GORM refuses unconditional UPDATEs without an explicit WHERE
		Update("storage_quota", quotaBytes)
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}
