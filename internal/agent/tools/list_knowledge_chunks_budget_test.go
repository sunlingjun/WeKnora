package tools

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Tencent/WeKnora/internal/types"
)

func TestResolveListChunksLimit(t *testing.T) {
	t.Parallel()

	if got := resolveListChunksLimit(0, "pdf"); got != listChunksDefaultLimit {
		t.Fatalf("pdf default = %d, want %d", got, listChunksDefaultLimit)
	}
	if got := resolveListChunksLimit(100, "pdf"); got != listChunksMaxLimit {
		t.Fatalf("pdf capped = %d, want %d", got, listChunksMaxLimit)
	}
	if got := resolveListChunksLimit(0, "xlsx"); got != listChunksTabularDefaultLimit {
		t.Fatalf("xlsx default = %d, want %d", got, listChunksTabularDefaultLimit)
	}
	if got := resolveListChunksLimit(50, "csv"); got != listChunksTabularMaxLimit {
		t.Fatalf("csv capped = %d, want %d", got, listChunksTabularMaxLimit)
	}
}

func TestSummarizeContentCapsTabularPreview(t *testing.T) {
	t.Parallel()

	huge := strings.Repeat("列值,", 2000)
	got := summarizeContent(huge, listChunksTabularContentCap)
	if utf8.RuneCountInString(got) > listChunksTabularContentCap+20 {
		t.Fatalf("tabular preview too long: %d runes", utf8.RuneCountInString(got))
	}
	if !strings.Contains(got, "...") {
		t.Fatalf("expected truncation marker, got %q", got[:min(80, len(got))])
	}
}

func TestBuildOutputUsesContentCap(t *testing.T) {
	t.Parallel()

	tool := &ListKnowledgeChunksTool{}
	chunks := []*types.Chunk{{
		ID:         "c1",
		ChunkIndex: 0,
		ChunkType:  types.ChunkTypeText,
		Content:    strings.Repeat("A", 5000),
	}}
	out := tool.buildOutput("d1", "sheet.xlsx", 1, 1, chunks, listChunksTabularContentCap)
	if !strings.Contains(out, "...") {
		t.Fatalf("expected truncated content in output:\n%s", out)
	}
	if strings.Count(out, "A") > listChunksTabularContentCap+50 {
		t.Fatalf("too many content runes leaked into output")
	}
}

func TestResolveKnowledgeFileTypeFromFileName(t *testing.T) {
	t.Parallel()

	got := resolveKnowledgeFileType(&types.Knowledge{FileName: "model_check_5.xlsx"})
	if got != "xlsx" {
		t.Fatalf("got %q, want xlsx", got)
	}
}

func TestListKnowledgeChunksBlocksCatalogIntent(t *testing.T) {
	t.Parallel()

	tool := &ListKnowledgeChunksTool{}
	ctx := WithUserQuery(context.Background(), "此知识库内有哪些知识？")
	result, err := tool.Execute(ctx, []byte(`{"knowledge_id":"d1"}`))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if result == nil || result.Success {
		t.Fatalf("expected catalog guard failure, got %+v", result)
	}
	if !strings.Contains(result.Error, "database_query") {
		t.Fatalf("expected guidance to use database_query, got %q", result.Error)
	}
}
