package agent

import (
	"errors"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/common"
	"github.com/Tencent/WeKnora/internal/types"
)

func TestIsContentPolicyError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{errors.New("timeout"), false},
		{errors.New(`API error: 400 {"error":{"message":"Content Exists Risk"}}`), true},
		{errors.New("finish_reason=content_filter"), true},
		{errors.New("DATA_INSPECTION_FAILED"), true},
		{errors.New("risk control failed for deploy"), false},
	}
	for _, tc := range cases {
		if got := isContentPolicyError(tc.err); got != tc.want {
			t.Fatalf("isContentPolicyError(%v)=%v want %v", tc.err, got, tc.want)
		}
	}
}

func TestBuildContentPolicyFallbackAnswer(t *testing.T) {
	t.Parallel()

	state := &types.AgentState{
		RoundSteps: []types.AgentStep{{
			ToolCalls: []types.ToolCall{
				{
					Name: "list_knowledge_chunks",
					Result: &types.ToolResult{
						Success: true,
						Output: `<knowledge_chunks knowledge_id="d1" title="model_check_5.xlsx" total="10" fetched="3">` +
							`<chunk><content>关键词,分类,判定,模型
foo,违反社会主义核心价值观,不安全,4b
bar,政治敏感话题,不安全,8b
</content></chunk></knowledge_chunks>`,
						Data: map[string]interface{}{
							"knowledge_title": "model_check_5.xlsx",
						},
					},
				},
				{
					Name: "database_query",
					Result: &types.ToolResult{
						Success: true,
						Output:  `<name>学联网手册</name>`,
					},
				},
			},
		}},
	}

	got := buildContentPolicyFallbackAnswer("此知识库内有哪些知识？", state)
	if !strings.Contains(got, "根据已检索到的知识库材料") {
		t.Fatalf("expected knowledge-style reply, got: %s", got)
	}
	if strings.Contains(got, "Content Exists Risk") || strings.Contains(got, "API request failed") {
		t.Fatalf("must not leak provider error payload: %s", got)
	}
	if !strings.Contains(got, "model_check_5.xlsx") || !strings.Contains(got, "学联网手册") {
		t.Fatalf("expected document titles, got: %s", got)
	}
	// Must NOT invent political prose without tying to observed label_counts.
	if strings.Contains(got, "一份内容安全检查类表格材料，通常列举") {
		t.Fatalf("must not hardcode invented safety-check prose: %s", got)
	}
	if !strings.Contains(got, "label_counts=") && !strings.Contains(got, "columns_guess=") {
		t.Fatalf("expected structural signals in fallback, got: %s", got)
	}
}

func TestExtractStructuralSignals(t *testing.T) {
	t.Parallel()

	raw := `<chunk><content>关键词,分类,判定,模型
aaa,政治敏感话题,不安全,4b
bbb,政治敏感话题,不安全,8b
</content></chunk>`
	got := extractStructuralSignals(raw)
	if !strings.Contains(got, "columns_guess=") {
		t.Fatalf("expected columns_guess, got: %s", got)
	}
	if !strings.Contains(got, "unsafe=") || !strings.Contains(got, "cat_political=") {
		t.Fatalf("expected short-code label_counts, got: %s", got)
	}
	if strings.Contains(got, "不安全=") || strings.Contains(got, "政治敏感话题=") {
		t.Fatalf("sensitive category phrases must not appear in digests: %s", got)
	}
	if strings.Contains(got, "aaa") || strings.Contains(got, "bbb") {
		t.Fatalf("concrete keywords must not appear: %s", got)
	}
}

func TestRedactToolOutputForSafeDigest(t *testing.T) {
	t.Parallel()

	raw := `<knowledge_chunks title="model_check_5.xlsx"><chunk><content>` +
		strings.Repeat("敏感词条ABCDEF,", 200) +
		`</content></chunk></knowledge_chunks>`
	got := redactToolOutputForSafeDigest(raw)
	if !strings.Contains(got, "redacted") {
		t.Fatalf("expected redaction marker, got: %s", got)
	}
	if strings.Contains(got, "敏感词条") {
		t.Fatalf("sensitive tokens must not leak into digest: %s", got)
	}
}

func TestBuildSafeToolDigest(t *testing.T) {
	t.Parallel()

	got := buildSafeToolDigest("list_knowledge_chunks", &types.ToolResult{
		Success: true,
		Output: `<knowledge_chunks title="model_check_5.xlsx" total="10"><chunk><content>` +
			"关键词,分类,判定\nx,政治敏感话题,不安全\n" + strings.Repeat("y", 200) +
			`</content></chunk></knowledge_chunks>`,
		Data: map[string]interface{}{
			"knowledge_title": "model_check_5.xlsx",
			"total_chunks":    int64(10),
		},
	})
	if !strings.Contains(got, "model_check_5.xlsx") {
		t.Fatalf("expected title in digest: %s", got)
	}
	if !strings.Contains(got, "total_chunks=10") {
		t.Fatalf("expected chunk count in digest: %s", got)
	}
	if !strings.Contains(got, "label_counts=") {
		t.Fatalf("expected label_counts in digest: %s", got)
	}
}

func TestCatalogIntentShared(t *testing.T) {
	t.Parallel()
	if !common.IsCatalogIntent("此知识库内有哪些知识？") {
		t.Fatal("expected catalog intent")
	}
}
