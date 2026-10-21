package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

const galleryStoredURL = "http://localhost:9000/nxinweknora/images/a00fbdb2461240f99855168f2432b3c4.png"

type galleryImageRepo struct {
	interfaces.ChunkRepository
	owned  bool
	gotURL string
	gotKB  string
}

func (r *galleryImageRepo) HasImageAssetURL(_ context.Context, _ uint64, kbID, imageURL string) (bool, error) {
	r.gotKB = kbID
	r.gotURL = imageURL
	return r.owned, nil
}

type galleryImageChunks struct {
	interfaces.ChunkService
	repo interfaces.ChunkRepository
}

func (s *galleryImageChunks) GetRepository() interfaces.ChunkRepository { return s.repo }

type galleryImageFiles struct {
	interfaces.FileService
	gotPath string
	body    string
	err     error
}

func (f *galleryImageFiles) GetFile(_ context.Context, filePath string) (io.ReadCloser, error) {
	f.gotPath = filePath
	if f.err != nil {
		return nil, f.err
	}
	return io.NopCloser(strings.NewReader(f.body)), nil
}

func galleryImageContentURL(raw string) string {
	return "/knowledge-bases/kb-1/images/content?url=" + url.QueryEscape(raw)
}

func newGalleryImageRouter(repo *galleryImageRepo, files *galleryImageFiles) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.Use(func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(1))
		c.Set(types.UserIDContextKey.String(), "u-test")
		c.Next()
	})
	h := &KnowledgeHandler{
		kbService:    &stubFolderKBService{},
		chunkService: &galleryImageChunks{repo: repo},
		fileService:  files,
	}
	r.GET("/knowledge-bases/:id/images/content", h.GetGalleryImageContent)
	return r
}

func TestGetGalleryImageContent(t *testing.T) {
	t.Setenv("MINIO_BUCKET_NAME", "nxinweknora")
	repo := &galleryImageRepo{owned: true}
	files := &galleryImageFiles{body: "png-bytes"}
	router := newGalleryImageRouter(repo, files)

	req := httptest.NewRequest(http.MethodGet, galleryImageContentURL(galleryStoredURL), nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status %d body %s", resp.Code, resp.Body.String())
	}
	if repo.gotURL != galleryStoredURL || repo.gotKB != "kb-1" {
		t.Fatalf("lookup kb=%q url=%q", repo.gotKB, repo.gotURL)
	}
	if files.gotPath != "minio://nxinweknora/images/a00fbdb2461240f99855168f2432b3c4.png" {
		t.Fatalf("storage path %q", files.gotPath)
	}
	if resp.Body.String() != "png-bytes" {
		t.Fatalf("body %q", resp.Body.String())
	}
	if ct := resp.Header().Get("Content-Type"); !strings.HasPrefix(ct, "image/png") {
		t.Fatalf("content type %q", ct)
	}
}

func TestGetGalleryImageContentRejectsUnownedAndPublicURLs(t *testing.T) {
	t.Setenv("MINIO_BUCKET_NAME", "nxinweknora")
	repo := &galleryImageRepo{owned: false}
	files := &galleryImageFiles{body: "secret"}
	router := newGalleryImageRouter(repo, files)

	unowned := httptest.NewRequest(http.MethodGet, galleryImageContentURL(galleryStoredURL), nil)
	unownedResp := httptest.NewRecorder()
	router.ServeHTTP(unownedResp, unowned)
	if unownedResp.Code != http.StatusNotFound {
		t.Fatalf("unowned status %d", unownedResp.Code)
	}
	if files.gotPath != "" {
		t.Fatalf("unowned URL reached storage: %s", files.gotPath)
	}

	public := httptest.NewRequest(http.MethodGet, galleryImageContentURL("https://cdn.example.com/a.png"), nil)
	publicResp := httptest.NewRecorder()
	router.ServeHTTP(publicResp, public)
	if publicResp.Code != http.StatusBadRequest {
		t.Fatalf("public status %d", publicResp.Code)
	}
}
