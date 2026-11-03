package mydoc

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type memFiles struct {
	store map[string][]byte
}

func (m *memFiles) CheckConnectivity(context.Context) error { return nil }
func (m *memFiles) SaveFile(context.Context, *multipart.FileHeader, uint64, string) (string, error) {
	return "", nil
}
func (m *memFiles) SaveBytes(_ context.Context, data []byte, _ uint64, fileName string, _ bool) (string, error) {
	if m.store == nil {
		m.store = map[string][]byte{}
	}
	key := "mem://" + fileName + "-" + uuid.NewString()
	cp := make([]byte, len(data))
	copy(cp, data)
	m.store[key] = cp
	return key, nil
}
func (m *memFiles) GetFile(_ context.Context, filePath string) (io.ReadCloser, error) {
	b, ok := m.store[filePath]
	if !ok {
		return nil, io.EOF
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}
func (m *memFiles) GetFileURL(context.Context, string) (string, error) { return "", nil }
func (m *memFiles) DeleteFile(_ context.Context, filePath string) error {
	delete(m.store, filePath)
	return nil
}
func (m *memFiles) CopyFile(context.Context, string, uint64, string) (string, error) {
	return "", nil
}

func newTestService(t *testing.T) (*Service, *repository.MyDocRepository) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.MyDocNode{}, &types.MyDocFile{}, &types.MyDocShare{}, &types.MyDocTicket{}))
	repo := repository.NewMyDocRepository(db)
	return NewService(repo, &memFiles{}, nil), repo
}

func TestMyDocService_NonOwnerCannotRename(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	folder, err := svc.Mkdir(ctx, "owner", "", "root")
	require.NoError(t, err)
	_, err = svc.Rename(ctx, "other", folder.ID, "x")
	require.ErrorIs(t, err, ErrNotOwner)
}

func TestMyDocService_MoveRejectsCycle(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	a, err := svc.Mkdir(ctx, "u1", "", "A")
	require.NoError(t, err)
	b, err := svc.Mkdir(ctx, "u1", a.ID, "B")
	require.NoError(t, err)
	_, err = svc.Move(ctx, "u1", a.ID, b.ID)
	require.ErrorIs(t, err, ErrCycle)
}

func TestMyDocService_ArchiveHiddenFromTree(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	f, err := svc.Mkdir(ctx, "u1", "", "gone")
	require.NoError(t, err)
	require.NoError(t, svc.Archive(ctx, "u1", f.ID))
	tree, err := svc.GetTree(ctx, "u1", types.MyDocViewOwned)
	require.NoError(t, err)
	require.Empty(t, tree)
}

func TestMyDocService_WriteTextBumpsVersion(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	item, err := svc.UploadFile(ctx, "u1", "", types.MyDocKindUpload, "note.md", "text/markdown", []byte("v1"), "", "")
	require.NoError(t, err)
	require.Equal(t, 1, item.File.Version)
	updated, err := svc.WriteText(ctx, "u1", item.ID, "v2", "agent-x")
	require.NoError(t, err)
	require.Equal(t, 2, updated.File.Version)
}

func TestMyDocService_ShareAndSharedTree(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	item, err := svc.UploadFile(ctx, "owner", "", types.MyDocKindUpload, "a.md", "text/markdown", []byte("hi"), "", "")
	require.NoError(t, err)
	share, err := svc.CreateShare(ctx, "owner", item.ID, "viewer")
	require.NoError(t, err)
	require.NotEmpty(t, share.ID)
	tree, err := svc.GetTree(ctx, "viewer", types.MyDocViewShared)
	require.NoError(t, err)
	require.Len(t, tree, 1)
	require.Equal(t, item.ID, tree[0].ID)
	_, err = svc.WriteText(ctx, "viewer", item.ID, "hack", "")
	require.ErrorIs(t, err, ErrNotOwner)
}

func TestRequireHumanUserRejectsMCP(t *testing.T) {
	ctx := types.WithPrincipal(context.Background(), types.MCPEndpointPrincipal(1, "ep"))
	ctx = context.WithValue(ctx, types.UserIDContextKey, "mcp-ep")
	_, err := RequireHumanUser(ctx)
	require.ErrorIs(t, err, ErrMachineCaller)
}

func TestMyDocService_DuplicateName(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	_, err := svc.Mkdir(ctx, "u1", "", "same")
	require.NoError(t, err)
	_, err = svc.Mkdir(ctx, "u1", "", "same")
	require.ErrorIs(t, err, ErrDuplicateName)
	_ = time.Now()
}
