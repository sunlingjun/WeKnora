package mydoc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"path/filepath"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	secutils "github.com/Tencent/WeKnora/internal/utils"
	"github.com/google/uuid"
)

// Service implements the MyDoc personal document hub.
type Service struct {
	repo     *repository.MyDocRepository
	files    interfaces.FileService
	messages interfaces.MessageService
}

// NewService constructs a MyDoc service.
func NewService(repo *repository.MyDocRepository, files interfaces.FileService, messages interfaces.MessageService) *Service {
	return &Service{repo: repo, files: files, messages: messages}
}

func storageTenantID(ctx context.Context) uint64 {
	if tid, ok := types.TenantIDFromContext(ctx); ok {
		return tid
	}
	return 0
}

func normalizeParentID(parentID string) *string {
	parentID = strings.TrimSpace(parentID)
	if parentID == "" {
		return nil
	}
	return &parentID
}

func (s *Service) ensureWritableParent(ctx context.Context, userID, parentID string) error {
	pid := strings.TrimSpace(parentID)
	if pid == "" {
		return nil
	}
	parent, err := s.repo.GetNode(ctx, pid)
	if err != nil {
		return err
	}
	if parent.OwnerUserID != userID || parent.Archived || parent.Type != types.MyDocNodeFolder {
		return ErrForbidden
	}
	return nil
}

func (s *Service) assertUniqueName(ctx context.Context, ownerUserID string, parentID *string, name, excludeID string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("%w: empty name", ErrInvalidInput)
	}
	dup, err := s.repo.FindSiblingByName(ctx, ownerUserID, parentID, name, excludeID)
	if err != nil {
		return err
	}
	if dup != nil {
		return &DuplicateNameError{Name: name, Kind: dup.Type}
	}
	return nil
}

// GetTree returns owned tree or shared flat list.
func (s *Service) GetTree(ctx context.Context, userID, view string) ([]*types.MyDocTreeNode, error) {
	view = strings.TrimSpace(view)
	if view == "" {
		view = types.MyDocViewOwned
	}
	switch view {
	case types.MyDocViewOwned:
		nodes, err := s.repo.ListOwned(ctx, userID, false)
		if err != nil {
			return nil, err
		}
		return s.buildTree(ctx, nodes)
	case types.MyDocViewShared:
		shares, err := s.repo.ListSharesByGrantee(ctx, userID)
		if err != nil {
			return nil, err
		}
		out := make([]*types.MyDocTreeNode, 0, len(shares))
		for _, sh := range shares {
			node, err := s.repo.GetNode(ctx, sh.DocID)
			if err != nil {
				if errors.Is(err, repository.ErrMyDocNotFound) {
					continue
				}
				return nil, err
			}
			if node.Archived || node.Type != types.MyDocNodeFile {
				continue
			}
			item := &types.MyDocTreeNode{MyDocNode: *node}
			if f, err := s.repo.GetFile(ctx, node.ID); err == nil {
				item.File = f
			}
			out = append(out, item)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("%w: unknown view", ErrInvalidInput)
	}
}

func (s *Service) buildTree(ctx context.Context, nodes []types.MyDocNode) ([]*types.MyDocTreeNode, error) {
	ids := make([]string, 0, len(nodes))
	for _, n := range nodes {
		if n.Type == types.MyDocNodeFile {
			ids = append(ids, n.ID)
		}
	}
	files, err := s.repo.GetFilesByNodeIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]*types.MyDocTreeNode, len(nodes))
	for i := range nodes {
		n := nodes[i]
		item := &types.MyDocTreeNode{MyDocNode: n, Children: []*types.MyDocTreeNode{}}
		if n.Type == types.MyDocNodeFile {
			item.File = files[n.ID]
		}
		byID[n.ID] = item
	}
	var roots []*types.MyDocTreeNode
	for _, item := range byID {
		if item.ParentID == nil || *item.ParentID == "" {
			roots = append(roots, item)
			continue
		}
		parent, ok := byID[*item.ParentID]
		if !ok {
			roots = append(roots, item)
			continue
		}
		parent.Children = append(parent.Children, item)
	}
	return roots, nil
}

// Mkdir creates a folder under parent (empty = root).
func (s *Service) Mkdir(ctx context.Context, userID, parentID, name string) (*types.MyDocNode, error) {
	name = strings.TrimSpace(name)
	if err := s.ensureWritableParent(ctx, userID, parentID); err != nil {
		return nil, err
	}
	pid := normalizeParentID(parentID)
	if err := s.assertUniqueName(ctx, userID, pid, name, ""); err != nil {
		return nil, err
	}
	now := time.Now()
	node := &types.MyDocNode{
		ID:          uuid.NewString(),
		OwnerUserID: userID,
		ParentID:    pid,
		Name:        name,
		Type:        types.MyDocNodeFolder,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.repo.CreateNode(ctx, node); err != nil {
		return nil, err
	}
	return node, nil
}

// Rename renames a node (owner only).
func (s *Service) Rename(ctx context.Context, userID, nodeID, name string) (*types.MyDocNode, error) {
	node, err := s.repo.GetNode(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	if !s.CanWrite(userID, node) {
		return nil, ErrNotOwner
	}
	name = strings.TrimSpace(name)
	if err := s.assertUniqueName(ctx, userID, node.ParentID, name, node.ID); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateNodeName(ctx, node.ID, name); err != nil {
		return nil, err
	}
	node.Name = name
	return node, nil
}

// Move moves a node under a new parent with cycle detection.
func (s *Service) Move(ctx context.Context, userID, nodeID, newParentID string) (*types.MyDocNode, error) {
	node, err := s.repo.GetNode(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	if !s.CanWrite(userID, node) {
		return nil, ErrNotOwner
	}
	if err := s.ensureWritableParent(ctx, userID, newParentID); err != nil {
		return nil, err
	}
	pid := normalizeParentID(newParentID)
	if pid != nil {
		isAnc, err := s.repo.IsAncestor(ctx, node.ID, *pid)
		if err != nil {
			return nil, err
		}
		if isAnc || *pid == node.ID {
			return nil, ErrCycle
		}
	}
	if err := s.assertUniqueName(ctx, userID, pid, node.Name, node.ID); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateParent(ctx, node.ID, pid); err != nil {
		return nil, err
	}
	node.ParentID = pid
	return node, nil
}

// BatchMove moves multiple nodes under the same parent.
func (s *Service) BatchMove(ctx context.Context, userID string, nodeIDs []string, newParentID string) error {
	for _, id := range nodeIDs {
		if _, err := s.Move(ctx, userID, id, newParentID); err != nil {
			return err
		}
	}
	return nil
}

// Archive hides a node and its descendants.
func (s *Service) Archive(ctx context.Context, userID, nodeID string) error {
	node, err := s.repo.GetNode(ctx, nodeID)
	if err != nil {
		return err
	}
	if !s.CanWrite(userID, node) {
		return ErrNotOwner
	}
	return s.repo.ArchiveSubtree(ctx, userID, nodeID)
}

// UploadFile stores bytes and creates a file node.
func (s *Service) UploadFile(
	ctx context.Context,
	userID, parentID, kind, filename, mimeType string,
	data []byte,
	sourceAgent, sourceSession string,
) (*types.MyDocTreeNode, error) {
	if err := s.ensureWritableParent(ctx, userID, parentID); err != nil {
		return nil, err
	}
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return nil, fmt.Errorf("%w: filename required", ErrInvalidInput)
	}
	kind = strings.TrimSpace(kind)
	if kind == "" {
		kind = types.MyDocKindUpload
	}
	switch kind {
	case types.MyDocKindUpload, types.MyDocKindAIArtifact, types.MyDocKindQASnapshot:
	default:
		return nil, fmt.Errorf("%w: invalid kind", ErrInvalidInput)
	}
	pid := normalizeParentID(parentID)
	if err := s.assertUniqueName(ctx, userID, pid, filename, ""); err != nil {
		return nil, err
	}
	if mimeType == "" {
		mimeType = mime.TypeByExtension(filepath.Ext(filename))
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
	}
	nodeID := uuid.NewString()
	storageName := fmt.Sprintf("mydoc_%s_v1%s", nodeID, filepath.Ext(filename))
	storageKey, err := s.files.SaveBytes(ctx, data, storageTenantID(ctx), storageName, false)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	now := time.Now()
	node := &types.MyDocNode{
		ID:          nodeID,
		OwnerUserID: userID,
		ParentID:    pid,
		Name:        filename,
		Type:        types.MyDocNodeFile,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	file := &types.MyDocFile{
		NodeID:        nodeID,
		Kind:          kind,
		StorageKey:    storageKey,
		Mime:          mimeType,
		Size:          int64(len(data)),
		ContentHash:   hex.EncodeToString(sum[:]),
		Version:       1,
		SourceAgent:   strings.TrimSpace(sourceAgent),
		SourceSession: strings.TrimSpace(sourceSession),
		Extra:         types.JSON([]byte("{}")),
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.repo.CreateNode(ctx, node); err != nil {
		_ = s.files.DeleteFile(ctx, storageKey)
		return nil, err
	}
	if err := s.repo.CreateFile(ctx, file); err != nil {
		_ = s.files.DeleteFile(ctx, storageKey)
		return nil, err
	}
	return &types.MyDocTreeNode{MyDocNode: *node, File: file}, nil
}

// WriteText creates or updates a text/markdown file for the owner.
func (s *Service) WriteText(ctx context.Context, userID, nodeID, content, sourceAgent string) (*types.MyDocTreeNode, error) {
	node, err := s.repo.GetNode(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	if !s.CanWrite(userID, node) {
		return nil, ErrNotOwner
	}
	if node.Type != types.MyDocNodeFile {
		return nil, ErrNotText
	}
	file, err := s.repo.GetFile(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	if !isEditableMime(file.Mime, node.Name) {
		return nil, ErrNotText
	}
	data := []byte(content)
	nextVer := file.Version + 1
	storageName := fmt.Sprintf("mydoc_%s_v%d%s", nodeID, nextVer, filepath.Ext(node.Name))
	storageKey, err := s.files.SaveBytes(ctx, data, storageTenantID(ctx), storageName, false)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	if err := s.repo.UpdateFileContentMeta(ctx, nodeID, storageKey, file.Mime, hex.EncodeToString(sum[:]), int64(len(data)), nextVer); err != nil {
		_ = s.files.DeleteFile(ctx, storageKey)
		return nil, err
	}
	if sourceAgent != "" {
		file.SourceAgent = sourceAgent
	}
	file.StorageKey = storageKey
	file.Size = int64(len(data))
	file.ContentHash = hex.EncodeToString(sum[:])
	file.Version = nextVer
	return &types.MyDocTreeNode{MyDocNode: *node, File: file}, nil
}

func isEditableMime(mimeType, name string) bool {
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".md", ".markdown", ".txt", ".text", ".json", ".csv", ".yaml", ".yml", ".xml", ".html", ".htm":
		return true
	}
	return strings.HasPrefix(mimeType, "text/") ||
		mimeType == "application/json" ||
		mimeType == "application/xml"
}

// GetFileMeta returns node+file if the caller can read.
func (s *Service) GetFileMeta(ctx context.Context, userID, nodeID, ticket string) (*types.MyDocTreeNode, error) {
	node, err := s.repo.GetNode(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	ok, err := s.CanRead(ctx, userID, node, ticket)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrForbidden
	}
	if node.Type != types.MyDocNodeFile {
		return &types.MyDocTreeNode{MyDocNode: *node}, nil
	}
	file, err := s.repo.GetFile(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	return &types.MyDocTreeNode{MyDocNode: *node, File: file}, nil
}

// ReadContent streams file bytes after ACL check.
func (s *Service) ReadContent(ctx context.Context, userID, nodeID, ticket string) (io.ReadCloser, *types.MyDocNode, *types.MyDocFile, error) {
	item, err := s.GetFileMeta(ctx, userID, nodeID, ticket)
	if err != nil {
		return nil, nil, nil, err
	}
	if item.Type != types.MyDocNodeFile || item.File == nil {
		return nil, nil, nil, ErrNotFound
	}
	rc, err := s.files.GetFile(ctx, item.File.StorageKey)
	if err != nil {
		return nil, nil, nil, err
	}
	return rc, &item.MyDocNode, item.File, nil
}

// CreateShare grants viewer access.
func (s *Service) CreateShare(ctx context.Context, userID, docID, granteeUserID string) (*types.MyDocShare, error) {
	node, err := s.repo.GetNode(ctx, docID)
	if err != nil {
		return nil, err
	}
	if !s.CanShare(userID, node) {
		return nil, ErrNotOwner
	}
	if node.Type != types.MyDocNodeFile {
		return nil, fmt.Errorf("%w: only files can be shared in P0", ErrInvalidInput)
	}
	granteeUserID = strings.TrimSpace(granteeUserID)
	if granteeUserID == "" || granteeUserID == userID {
		return nil, fmt.Errorf("%w: invalid grantee", ErrInvalidInput)
	}
	if existing, err := s.repo.GetShareByDocGrantee(ctx, docID, granteeUserID); err != nil {
		return nil, err
	} else if existing != nil {
		return existing, nil
	}
	share := &types.MyDocShare{
		ID:            uuid.NewString(),
		DocID:         docID,
		GranteeUserID: granteeUserID,
		Role:          types.MyDocRoleViewer,
		CreatedAt:     time.Now(),
	}
	if err := s.repo.CreateShare(ctx, share); err != nil {
		return nil, err
	}
	return share, nil
}

// DeleteShare removes a share and revokes tickets for that doc.
func (s *Service) DeleteShare(ctx context.Context, userID, shareID string) error {
	share, err := s.repo.GetShare(ctx, shareID)
	if err != nil {
		return err
	}
	node, err := s.repo.GetNode(ctx, share.DocID)
	if err != nil {
		return err
	}
	if !s.CanShare(userID, node) {
		return ErrNotOwner
	}
	if err := s.repo.DeleteShare(ctx, shareID); err != nil {
		return err
	}
	return s.repo.RevokeTicketsByDoc(ctx, share.DocID)
}

// ListShares returns shares for a doc (owner only).
func (s *Service) ListShares(ctx context.Context, userID, docID string) ([]types.MyDocShare, error) {
	node, err := s.repo.GetNode(ctx, docID)
	if err != nil {
		return nil, err
	}
	if !s.CanShare(userID, node) {
		return nil, ErrNotOwner
	}
	return s.repo.ListSharesByDoc(ctx, docID)
}

// CreateTicket mints a short ticket and persists its hash.
func (s *Service) CreateTicket(ctx context.Context, userID, docID, purpose string) (token string, row *types.MyDocTicket, err error) {
	node, err := s.repo.GetNode(ctx, docID)
	if err != nil {
		return "", nil, err
	}
	if !s.CanShare(userID, node) && !s.CanWrite(userID, node) {
		ok, err := s.CanRead(ctx, userID, node, "")
		if err != nil {
			return "", nil, err
		}
		if !ok {
			return "", nil, ErrForbidden
		}
	}
	purpose = strings.TrimSpace(purpose)
	if purpose == "" {
		purpose = types.MyDocTicketDownload
	}
	if purpose != types.MyDocTicketDownload && purpose != types.MyDocTicketPreview {
		return "", nil, fmt.Errorf("%w: invalid purpose", ErrInvalidInput)
	}
	token, expiresAt, err := secutils.SignMyDocTicket(docID, purpose, time.Now())
	if err != nil {
		return "", nil, err
	}
	row = &types.MyDocTicket{
		ID:        uuid.NewString(),
		DocID:     docID,
		Purpose:   purpose,
		TokenHash: secutils.HashMyDocTicketToken(token),
		ExpiresAt: expiresAt,
		CreatedBy: userID,
		CreatedAt: time.Now(),
	}
	if err := s.repo.CreateTicket(ctx, row); err != nil {
		return "", nil, err
	}
	return token, row, nil
}

// RenewTicket re-signs within grace window.
func (s *Service) RenewTicket(ctx context.Context, ticketID, oldToken string) (token string, row *types.MyDocTicket, err error) {
	row, err = s.repo.GetTicket(ctx, ticketID)
	if err != nil {
		return "", nil, err
	}
	if row.Revoked {
		return "", nil, ErrForbidden
	}
	claims, err := secutils.ParseMyDocTicket(oldToken, time.Now(), true)
	if err != nil || claims.DocID != row.DocID {
		return "", nil, ErrForbidden
	}
	grace := secutils.TicketRenewGrace()
	if time.Now().After(row.ExpiresAt.Add(grace)) {
		return "", nil, ErrForbidden
	}
	token, expiresAt, err := secutils.SignMyDocTicket(row.DocID, row.Purpose, time.Now())
	if err != nil {
		return "", nil, err
	}
	hash := secutils.HashMyDocTicketToken(token)
	if err := s.repo.UpdateTicketExpiry(ctx, row.ID, expiresAt, hash); err != nil {
		return "", nil, err
	}
	row.ExpiresAt = expiresAt
	row.TokenHash = hash
	return token, row, nil
}

// CreateQASnapshot renders session messages to markdown and uploads as qa_snapshot.
func (s *Service) CreateQASnapshot(ctx context.Context, userID, parentID, sessionID, title string) (*types.MyDocTreeNode, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, fmt.Errorf("%w: session_id required", ErrInvalidInput)
	}
	if s.messages == nil {
		return nil, fmt.Errorf("%w: message service unavailable", ErrInvalidInput)
	}
	msgs, err := s.messages.GetMessagesBySession(ctx, sessionID, 1, 500)
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	if title == "" {
		title = "qa-" + sessionID
	}
	b.WriteString("# ")
	b.WriteString(title)
	b.WriteString("\n\n")
	for _, m := range msgs {
		if m == nil {
			continue
		}
		role := strings.TrimSpace(m.Role)
		if role == "" {
			role = "message"
		}
		b.WriteString("## ")
		b.WriteString(role)
		b.WriteString("\n\n")
		b.WriteString(strings.TrimSpace(m.Content))
		b.WriteString("\n\n")
	}
	filename := title
	if !strings.HasSuffix(strings.ToLower(filename), ".md") {
		filename += ".md"
	}
	return s.UploadFile(ctx, userID, parentID, types.MyDocKindQASnapshot, filename, "text/markdown", []byte(b.String()), "", sessionID)
}
