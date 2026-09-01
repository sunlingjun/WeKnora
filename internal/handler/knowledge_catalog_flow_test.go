package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type flowCatalogService struct {
	interfaces.KnowledgeCatalogService
}

func (s *flowCatalogService) ListAuthorizedCatalogKBs(
	_ context.Context,
	_ types.ListCatalogKBsQuery,
) (*types.CatalogKBListResult, error) {
	return &types.CatalogKBListResult{
		TenantID: 42,
		KnowledgeBases: []types.CatalogKnowledgeBase{
			{
				ID:             "kb-own-1",
				Name:           "产品文档",
				AccessSource:   types.CatalogAccessOwned,
				OwnerTenantID:  42,
				Permission:     string(types.OrgRoleAdmin),
				CanDownload:    true,
				KnowledgeCount: 2,
			},
			{
				ID:             "kb-share-9",
				Name:           "集团制度",
				AccessSource:   types.CatalogAccessOrgShare,
				OwnerTenantID:  7,
				OrganizationID: "org-1",
				ShareID:        "share-1",
				Permission:     string(types.OrgRoleViewer),
				CanDownload:    false,
				KnowledgeCount: 1,
			},
		},
		Total: 2,
	}, nil
}

func (s *flowCatalogService) ListCatalogKnowledge(
	_ context.Context,
	q types.ListCatalogKnowledgeQuery,
) (*types.CatalogKnowledgeListResult, error) {
	switch q.KBID {
	case "kb-own-1":
		if q.Cursor == "" {
			return &types.CatalogKnowledgeListResult{
				TenantID: 42,
				KBID:     q.KBID,
				Items: []types.CatalogKnowledgeItem{
					{ID: "k-file-1", KnowledgeBaseID: q.KBID, KnowledgeType: "file", Title: "报销制度.pdf", HasFile: true},
				},
				NextCursor: "cursor-page-2",
				HasMore:    true,
			}, nil
		}
		return &types.CatalogKnowledgeListResult{
			TenantID: 42,
			KBID:     q.KBID,
			Items: []types.CatalogKnowledgeItem{
				{ID: "k-manual-1", KnowledgeBaseID: q.KBID, KnowledgeType: "manual", Title: "手工条目", HasFile: false},
			},
		}, nil
	case "kb-share-9":
		return &types.CatalogKnowledgeListResult{
			TenantID: 42,
			KBID:     q.KBID,
			Items: []types.CatalogKnowledgeItem{
				{ID: "k-share-file", KnowledgeBaseID: q.KBID, KnowledgeType: "file", Title: "分享文件.pdf", HasFile: true},
			},
		}, nil
	default:
		return nil, service.ErrCatalogKBNotFound
	}
}

func newCatalogFlowRouter(t *testing.T) (*gin.Engine, *[]string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	downloads := &[]string{}
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	h := NewKnowledgeCatalogHandler(&flowCatalogService{})
	r.GET("/api/v1/knowledge-catalog/knowledge-bases", h.ListKnowledgeBases)
	r.GET("/api/v1/knowledge-catalog/knowledge", h.ListKnowledge)
	r.GET("/api/v1/knowledge/:id/download", func(c *gin.Context) {
		id := c.Param("id")
		if id != "k-file-1" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "error": gin.H{"message": "Forbidden"}})
			return
		}
		*downloads = append(*downloads, id)
		c.Header("Content-Disposition", `attachment; filename="报销制度.pdf"`)
		c.Data(http.StatusOK, "application/octet-stream", []byte("%PDF-flow-test"))
	})
	return r, downloads
}

func TestCatalogClientFlow_DownloadOnlyOwnedFiles(t *testing.T) {
	engine, downloads := newCatalogFlowRouter(t)
	server := httptest.NewServer(engine)
	t.Cleanup(server.Close)

	catalogBody := getJSON(t, server, "/api/v1/knowledge-catalog/knowledge-bases")
	require.NotContains(t, catalogBody, "file_path")
	require.NotContains(t, catalogBody, "vector_store_id")

	var catalog struct {
		Success bool `json:"success"`
		Data    struct {
			KnowledgeBases []struct {
				ID           string `json:"id"`
				CanDownload  bool   `json:"can_download"`
				AccessSource string `json:"access_source"`
			} `json:"knowledge_bases"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(catalogBody), &catalog))
	require.True(t, catalog.Success)
	require.Len(t, catalog.Data.KnowledgeBases, 2)

	var downloaded []string
	var skipped []string
	for _, kb := range catalog.Data.KnowledgeBases {
		cursor := ""
		pages := 0
		for {
			path := "/api/v1/knowledge-catalog/knowledge?kb_id=" + kb.ID + "&limit=1"
			if cursor != "" {
				path += "&cursor=" + cursor
			}
			pageBody := getJSON(t, server, path)
			require.NotContains(t, pageBody, "file_path")
			require.NotContains(t, pageBody, "vector_store_id")
			var page struct {
				Data struct {
					Items []struct {
						ID            string `json:"id"`
						KnowledgeType string `json:"knowledge_type"`
						HasFile       bool   `json:"has_file"`
					} `json:"items"`
					NextCursor string `json:"next_cursor"`
					HasMore    bool   `json:"has_more"`
				} `json:"data"`
			}
			require.NoError(t, json.Unmarshal([]byte(pageBody), &page))
			pages++
			for _, item := range page.Data.Items {
				if kb.CanDownload && item.KnowledgeType == "file" && item.HasFile {
					status, _ := getRaw(t, server, "/api/v1/knowledge/"+item.ID+"/download")
					require.Equal(t, http.StatusOK, status)
					downloaded = append(downloaded, item.ID)
					continue
				}
				skipped = append(skipped, item.ID)
			}
			if !page.Data.HasMore {
				break
			}
			cursor = page.Data.NextCursor
			require.NotEmpty(t, cursor)
			require.Less(t, pages, 5)
		}
	}

	require.Equal(t, []string{"k-file-1"}, downloaded)
	require.Equal(t, []string{"k-file-1"}, *downloads)
	require.ElementsMatch(t, []string{"k-manual-1", "k-share-file"}, skipped)
}

func getJSON(t *testing.T, server *httptest.Server, path string) string {
	t.Helper()
	status, body := getRaw(t, server, path)
	require.Equal(t, http.StatusOK, status, body)
	return body
}

func getRaw(t *testing.T, server *httptest.Server, path string) (int, string) {
	t.Helper()
	resp, err := http.Get(server.URL + path)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(raw)
}
