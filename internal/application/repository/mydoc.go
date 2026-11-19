package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/gorm"
)

var (
	// Stable English sentinels for errors.Is; UI copy lives in i18n (mydoc.errors.*).
	ErrMyDocNotFound      = errors.New("mydoc: not found")
	ErrMyDocDuplicateName = errors.New("mydoc: duplicate name under parent")
)

// MyDocRepository persists MyDoc nodes, files, shares and tickets.
type MyDocRepository struct {
	db *gorm.DB
}

// NewMyDocRepository creates a MyDoc repository.
func NewMyDocRepository(db *gorm.DB) *MyDocRepository {
	return &MyDocRepository{db: db}
}

func (r *MyDocRepository) CreateNode(ctx context.Context, node *types.MyDocNode) error {
	return r.db.WithContext(ctx).Create(node).Error
}

func (r *MyDocRepository) GetNode(ctx context.Context, id string) (*types.MyDocNode, error) {
	var node types.MyDocNode
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&node).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMyDocNotFound
	}
	if err != nil {
		return nil, err
	}
	return &node, nil
}

func (r *MyDocRepository) UpdateNodeName(ctx context.Context, id, name string) error {
	res := r.db.WithContext(ctx).Model(&types.MyDocNode{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{"name": name, "updated_at": time.Now()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrMyDocNotFound
	}
	return nil
}

func (r *MyDocRepository) UpdateParent(ctx context.Context, id string, parentID *string) error {
	res := r.db.WithContext(ctx).Model(&types.MyDocNode{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{"parent_id": parentID, "updated_at": time.Now()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrMyDocNotFound
	}
	return nil
}

func (r *MyDocRepository) FindSiblingByName(ctx context.Context, ownerUserID string, parentID *string, name string, excludeID string) (*types.MyDocNode, error) {
	q := r.db.WithContext(ctx).Model(&types.MyDocNode{}).
		Where("owner_user_id = ? AND name = ? AND archived = ?", ownerUserID, name, false)
	if parentID == nil || *parentID == "" {
		q = q.Where("parent_id IS NULL")
	} else {
		q = q.Where("parent_id = ?", *parentID)
	}
	if excludeID != "" {
		q = q.Where("id <> ?", excludeID)
	}
	var node types.MyDocNode
	err := q.First(&node).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &node, nil
}

func (r *MyDocRepository) ListChildren(ctx context.Context, ownerUserID string, parentID *string, includeArchived bool) ([]types.MyDocNode, error) {
	q := r.db.WithContext(ctx).Where("owner_user_id = ?", ownerUserID)
	if parentID == nil || *parentID == "" {
		q = q.Where("parent_id IS NULL")
	} else {
		q = q.Where("parent_id = ?", *parentID)
	}
	if !includeArchived {
		q = q.Where("archived = ?", false)
	}
	var nodes []types.MyDocNode
	if err := q.Order("type ASC, name ASC").Find(&nodes).Error; err != nil {
		return nil, err
	}
	return nodes, nil
}

func (r *MyDocRepository) ListOwned(ctx context.Context, ownerUserID string, includeArchived bool) ([]types.MyDocNode, error) {
	q := r.db.WithContext(ctx).Where("owner_user_id = ?", ownerUserID)
	if !includeArchived {
		q = q.Where("archived = ?", false)
	}
	var nodes []types.MyDocNode
	if err := q.Order("type ASC, name ASC").Find(&nodes).Error; err != nil {
		return nil, err
	}
	return nodes, nil
}

func (r *MyDocRepository) ArchiveSubtree(ctx context.Context, ownerUserID, rootID string) error {
	ids, err := r.collectDescendantIDs(ctx, ownerUserID, rootID)
	if err != nil {
		return err
	}
	ids = append(ids, rootID)
	now := time.Now()
	return r.db.WithContext(ctx).Model(&types.MyDocNode{}).
		Where("owner_user_id = ? AND id IN ?", ownerUserID, ids).
		Updates(map[string]interface{}{"archived": true, "archived_at": now, "updated_at": now}).Error
}

func (r *MyDocRepository) collectDescendantIDs(ctx context.Context, ownerUserID, rootID string) ([]string, error) {
	var out []string
	queue := []string{rootID}
	seen := map[string]struct{}{rootID: {}}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		var children []types.MyDocNode
		if err := r.db.WithContext(ctx).
			Select("id").
			Where("owner_user_id = ? AND parent_id = ?", ownerUserID, cur).
			Find(&children).Error; err != nil {
			return nil, err
		}
		for _, ch := range children {
			if _, ok := seen[ch.ID]; ok {
				continue
			}
			seen[ch.ID] = struct{}{}
			out = append(out, ch.ID)
			queue = append(queue, ch.ID)
		}
	}
	return out, nil
}

// IsAncestor reports whether ancestorID is an ancestor of nodeID (or equal).
func (r *MyDocRepository) IsAncestor(ctx context.Context, ancestorID, nodeID string) (bool, error) {
	if ancestorID == "" || nodeID == "" {
		return false, nil
	}
	if ancestorID == nodeID {
		return true, nil
	}
	cur := nodeID
	for i := 0; i < 256; i++ {
		node, err := r.GetNode(ctx, cur)
		if err != nil {
			return false, err
		}
		if node.ParentID == nil || *node.ParentID == "" {
			return false, nil
		}
		if *node.ParentID == ancestorID {
			return true, nil
		}
		cur = *node.ParentID
	}
	return false, fmt.Errorf("mydoc: parent chain too deep")
}

func (r *MyDocRepository) CreateFile(ctx context.Context, file *types.MyDocFile) error {
	return r.db.WithContext(ctx).Create(file).Error
}

func (r *MyDocRepository) GetFile(ctx context.Context, nodeID string) (*types.MyDocFile, error) {
	var file types.MyDocFile
	err := r.db.WithContext(ctx).Where("node_id = ?", nodeID).First(&file).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMyDocNotFound
	}
	if err != nil {
		return nil, err
	}
	return &file, nil
}

func (r *MyDocRepository) GetFilesByNodeIDs(ctx context.Context, nodeIDs []string) (map[string]*types.MyDocFile, error) {
	out := make(map[string]*types.MyDocFile, len(nodeIDs))
	if len(nodeIDs) == 0 {
		return out, nil
	}
	var files []types.MyDocFile
	if err := r.db.WithContext(ctx).Where("node_id IN ?", nodeIDs).Find(&files).Error; err != nil {
		return nil, err
	}
	for i := range files {
		f := files[i]
		out[f.NodeID] = &f
	}
	return out, nil
}

func (r *MyDocRepository) UpdateFileContentMeta(ctx context.Context, nodeID, storageKey, mime, contentHash string, size int64, version int) error {
	res := r.db.WithContext(ctx).Model(&types.MyDocFile{}).
		Where("node_id = ?", nodeID).
		Updates(map[string]interface{}{
			"storage_key":  storageKey,
			"mime":         mime,
			"content_hash": contentHash,
			"size":         size,
			"version":      version,
			"updated_at":   time.Now(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrMyDocNotFound
	}
	return nil
}

func (r *MyDocRepository) CreateShare(ctx context.Context, share *types.MyDocShare) error {
	return r.db.WithContext(ctx).Create(share).Error
}

func (r *MyDocRepository) DeleteShare(ctx context.Context, shareID string) error {
	res := r.db.WithContext(ctx).Where("id = ?", shareID).Delete(&types.MyDocShare{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrMyDocNotFound
	}
	return nil
}

func (r *MyDocRepository) GetShare(ctx context.Context, shareID string) (*types.MyDocShare, error) {
	var share types.MyDocShare
	err := r.db.WithContext(ctx).Where("id = ?", shareID).First(&share).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMyDocNotFound
	}
	if err != nil {
		return nil, err
	}
	return &share, nil
}

func (r *MyDocRepository) GetShareByDocGrantee(ctx context.Context, docID, granteeUserID string) (*types.MyDocShare, error) {
	var share types.MyDocShare
	err := r.db.WithContext(ctx).
		Where("doc_id = ? AND grantee_user_id = ?", docID, granteeUserID).
		First(&share).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &share, nil
}

func (r *MyDocRepository) ListSharesByGrantee(ctx context.Context, granteeUserID string) ([]types.MyDocShare, error) {
	var shares []types.MyDocShare
	if err := r.db.WithContext(ctx).Where("grantee_user_id = ?", granteeUserID).Find(&shares).Error; err != nil {
		return nil, err
	}
	return shares, nil
}

func (r *MyDocRepository) ListSharesByDoc(ctx context.Context, docID string) ([]types.MyDocShare, error) {
	var shares []types.MyDocShare
	if err := r.db.WithContext(ctx).Where("doc_id = ?", docID).Find(&shares).Error; err != nil {
		return nil, err
	}
	return shares, nil
}

func (r *MyDocRepository) CreateTicket(ctx context.Context, ticket *types.MyDocTicket) error {
	return r.db.WithContext(ctx).Create(ticket).Error
}

func (r *MyDocRepository) GetTicketByHash(ctx context.Context, tokenHash string) (*types.MyDocTicket, error) {
	var ticket types.MyDocTicket
	err := r.db.WithContext(ctx).Where("token_hash = ?", tokenHash).First(&ticket).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMyDocNotFound
	}
	if err != nil {
		return nil, err
	}
	return &ticket, nil
}

func (r *MyDocRepository) GetTicket(ctx context.Context, id string) (*types.MyDocTicket, error) {
	var ticket types.MyDocTicket
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&ticket).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMyDocNotFound
	}
	if err != nil {
		return nil, err
	}
	return &ticket, nil
}

func (r *MyDocRepository) RevokeTicketsByDoc(ctx context.Context, docID string) error {
	return r.db.WithContext(ctx).Model(&types.MyDocTicket{}).
		Where("doc_id = ? AND revoked = ?", docID, false).
		Update("revoked", true).Error
}

func (r *MyDocRepository) UpdateTicketExpiry(ctx context.Context, id string, expiresAt time.Time, tokenHash string) error {
	return r.db.WithContext(ctx).Model(&types.MyDocTicket{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"expires_at": expiresAt,
			"token_hash": tokenHash,
			"revoked":    false,
		}).Error
}
