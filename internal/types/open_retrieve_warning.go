package types

import (
	"context"
	"sync"
)

// OpenRetrieveWarning is one knowledge-base group that open retrieve could
// not search completely. Other groups' hits are still returned.
type OpenRetrieveWarning struct {
	KnowledgeBaseIDs []string `json:"knowledge_base_ids"`
	Reason           string   `json:"reason"`
}

// OpenRetrieveWarningCollector accumulates warnings for one retrieve call.
type OpenRetrieveWarningCollector struct {
	mu    sync.Mutex
	items []OpenRetrieveWarning
}

// WithOpenRetrieveWarningCollector attaches a collector to ctx and returns it.
// The handler keeps the pointer and reads Items after SearchKnowledgeOpen.
func WithOpenRetrieveWarningCollector(ctx context.Context) (context.Context, *OpenRetrieveWarningCollector) {
	box := &OpenRetrieveWarningCollector{}
	return context.WithValue(ctx, OpenRetrieveWarningContextKey, box), box
}

// OpenRetrieveWarningCollectorFromContext returns the collector on ctx, or nil.
func OpenRetrieveWarningCollectorFromContext(ctx context.Context) *OpenRetrieveWarningCollector {
	if ctx == nil {
		return nil
	}
	box, _ := ctx.Value(OpenRetrieveWarningContextKey).(*OpenRetrieveWarningCollector)
	return box
}

// Add records one skipped or degraded group. kbIDs is copied.
func (c *OpenRetrieveWarningCollector) Add(kbIDs []string, reason string) {
	if c == nil || reason == "" {
		return
	}
	ids := append([]string(nil), kbIDs...)
	c.mu.Lock()
	c.items = append(c.items, OpenRetrieveWarning{
		KnowledgeBaseIDs: ids,
		Reason:           reason,
	})
	c.mu.Unlock()
}

// Items returns a copy of the warnings recorded so far.
func (c *OpenRetrieveWarningCollector) Items() []OpenRetrieveWarning {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.items) == 0 {
		return nil
	}
	out := make([]OpenRetrieveWarning, len(c.items))
	for i, item := range c.items {
		out[i] = OpenRetrieveWarning{
			KnowledgeBaseIDs: append([]string(nil), item.KnowledgeBaseIDs...),
			Reason:           item.Reason,
		}
	}
	return out
}
