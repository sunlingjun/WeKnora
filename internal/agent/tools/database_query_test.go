package tools

import (
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
)

func TestDatabaseQueryInjectsEnabledChunkFilter(t *testing.T) {
	tool := NewDatabaseQueryTool(nil, types.SearchTargets{{
		Type:            types.SearchTargetTypeKnowledgeBase,
		KnowledgeBaseID: "kb-1",
		TenantID:        1,
	}})

	securedSQL, err := tool.validateAndSecureSQL(
		"SELECT c.id, c.content FROM chunks c WHERE c.chunk_type = 'faq'",
		1,
	)
	if err != nil {
		t.Fatalf("validateAndSecureSQL() error = %v", err)
	}
	if !strings.Contains(securedSQL, "c.is_enabled = true") {
		t.Fatalf("Agent SQL must exclude disabled chunks:\n%s", securedSQL)
	}
}

func TestDatabaseQueryUsesOwnerTenantForPlazaKB(t *testing.T) {
	tool := NewDatabaseQueryTool(nil, types.SearchTargets{{
		Type:            types.SearchTargetTypeKnowledgeBase,
		KnowledgeBaseID: "dd98ab62-b66d-49f8-89f9-d3c3ab55cb8f",
		TenantID:        10038,
	}})

	securedSQL, err := tool.validateAndSecureSQL(
		"SELECT id, title FROM knowledges",
		10035,
	)
	if err != nil {
		t.Fatalf("validateAndSecureSQL() error = %v", err)
	}
	if !strings.Contains(securedSQL, "knowledges.tenant_id = 10038") {
		t.Fatalf("plaza KB SQL must use owner tenant, got:\n%s", securedSQL)
	}
	if strings.Contains(securedSQL, "knowledges.tenant_id = 10035") {
		t.Fatalf("plaza KB SQL must not isolate to caller tenant only:\n%s", securedSQL)
	}
}
