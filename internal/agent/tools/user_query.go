package tools

import "context"

type userQueryKey struct{}

// WithUserQuery publishes the current user turn query for tool-side guards
// (e.g. catalog intent must not deep-read document bodies).
func WithUserQuery(ctx context.Context, query string) context.Context {
	if ctx == nil || query == "" {
		return ctx
	}
	return context.WithValue(ctx, userQueryKey{}, query)
}

// UserQueryFromContext returns the current user query if present.
func UserQueryFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if q, ok := ctx.Value(userQueryKey{}).(string); ok {
		return q
	}
	if meta, ok := ToolExecFromContext(ctx); ok && meta != nil {
		return meta.UserQuery
	}
	return ""
}
