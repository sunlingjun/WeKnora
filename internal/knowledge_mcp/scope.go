package knowledge_mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/Tencent/WeKnora/internal/types"
)

// ScopeError is a tool-level error code for KB scope resolution.
type ScopeError struct {
	Code    string
	Message string
}

func (e *ScopeError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

const (
	codeKBOutOfScope        = "kb_out_of_scope"
	codeEmptyKBIntersection = "empty_kb_intersection"
	codeNoKBInScope         = "no_kb_in_scope"
	codeNoWikiKBInScope     = "no_wiki_kb_in_scope"
	codeInvalidLimit        = "invalid_limit"
	codeMissingQuery        = "missing_query"
	codeInvalidMode         = "invalid_mode"
	codeMissingCenter       = "missing_center"
)

// KBSummary is the public KB metadata returned by kb_list / used internally.
type KBSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	WikiEnabled bool   `json:"wiki_enabled"`
}

type kbLister interface {
	ListKnowledgeBases(ctx context.Context) ([]*types.KnowledgeBase, error)
}

type scopeResolver struct {
	kb kbLister
}

func newScopeResolver(kb kbLister) *scopeResolver {
	return &scopeResolver{kb: kb}
}

// listAuthorized returns KBs visible to the current tenant, filtered by API key
// KnowledgeBaseIDs when the key is restricted.
func (r *scopeResolver) listAuthorized(ctx context.Context) ([]*types.KnowledgeBase, error) {
	all, err := r.kb.ListKnowledgeBases(ctx)
	if err != nil {
		return nil, err
	}
	scope, ok := types.TenantAPIKeyScopeFromContext(ctx)
	if !ok || !scope.IsKnowledgeBaseRestricted() {
		return all, nil
	}
	allowed := make(map[string]struct{}, len(scope.KnowledgeBaseIDs))
	for _, id := range scope.KnowledgeBaseIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		allowed[id] = struct{}{}
	}
	out := make([]*types.KnowledgeBase, 0, len(allowed))
	for _, kb := range all {
		if kb == nil {
			continue
		}
		if _, hit := allowed[kb.ID]; hit {
			out = append(out, kb)
		}
	}
	return out, nil
}

// resolveEffective applies authorized ∩ requested with strict out-of-scope errors.
func (r *scopeResolver) resolveEffective(ctx context.Context, requested []string) ([]*types.KnowledgeBase, error) {
	authorized, err := r.listAuthorized(ctx)
	if err != nil {
		return nil, err
	}
	if len(authorized) == 0 {
		return nil, &ScopeError{Code: codeNoKBInScope, Message: "no knowledge base in API key scope"}
	}

	requested = normalizeIDs(requested)
	if len(requested) == 0 {
		return authorized, nil
	}

	authSet := make(map[string]*types.KnowledgeBase, len(authorized))
	for _, kb := range authorized {
		authSet[kb.ID] = kb
	}

	var outOfScope []string
	effective := make([]*types.KnowledgeBase, 0, len(requested))
	seen := map[string]struct{}{}
	for _, id := range requested {
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		kb, ok := authSet[id]
		if !ok {
			outOfScope = append(outOfScope, id)
			continue
		}
		effective = append(effective, kb)
	}
	if len(outOfScope) > 0 {
		return nil, &ScopeError{
			Code:    codeKBOutOfScope,
			Message: fmt.Sprintf("knowledge base id(s) not in API key scope: %s", strings.Join(outOfScope, ", ")),
		}
	}
	if len(effective) == 0 {
		return nil, &ScopeError{Code: codeEmptyKBIntersection, Message: "intersection of kb_list and API key scope is empty"}
	}
	return effective, nil
}

func filterWikiEnabled(kbs []*types.KnowledgeBase) []*types.KnowledgeBase {
	out := make([]*types.KnowledgeBase, 0, len(kbs))
	for _, kb := range kbs {
		if kb != nil && kb.IsWikiEnabled() {
			out = append(out, kb)
		}
	}
	return out
}

func toSummaries(kbs []*types.KnowledgeBase) []KBSummary {
	out := make([]KBSummary, 0, len(kbs))
	for _, kb := range kbs {
		if kb == nil {
			continue
		}
		out = append(out, KBSummary{
			ID:          kb.ID,
			Name:        kb.Name,
			Type:        kb.Type,
			WikiEnabled: kb.IsWikiEnabled(),
		})
	}
	return out
}

func normalizeIDs(ids []string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		out = append(out, id)
	}
	return out
}
