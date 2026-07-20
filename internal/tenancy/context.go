// Package tenancy provides the tenant-context plumbing for the hybrid
// multi-tenant model: request contexts carry the current organization ID, and
// every database transaction is opened with the matching Postgres GUC so that
// row-level security policies scope all reads and writes to that tenant.
package tenancy

import "context"

// contextKey is an unexported type that prevents context key collisions across packages.
type contextKey string

// tenantIDKey is the context key under which the current organization ID is stored.
const tenantIDKey contextKey = "tenant_id"

// WithTenantID returns a child context carrying the given organization ID.
func WithTenantID(ctx context.Context, orgID string) context.Context {
	return context.WithValue(ctx, tenantIDKey, orgID)
}

// TenantIDFromContext extracts the organization ID injected by WithTenantID.
func TenantIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(tenantIDKey).(string)
	return id, ok && id != ""
}
