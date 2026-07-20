package tenancy

import (
	"context"
	"testing"
)

func TestTenantIDFromContext(t *testing.T) {
	const orgID = "11111111-1111-1111-1111-111111111111"

	// Equivalence partitions: enriched context, bare context, empty-string
	// tenant, and a foreign value stored under a colliding string key.
	tests := []struct {
		name   string
		ctx    context.Context
		wantID string
		wantOK bool
	}{
		{
			name:   "round_trip_returns_stored_org_id",
			ctx:    WithTenantID(context.Background(), orgID),
			wantID: orgID,
			wantOK: true,
		},
		{
			name:   "bare_context_returns_not_ok",
			ctx:    context.Background(),
			wantID: "",
			wantOK: false,
		},
		{
			name:   "empty_tenant_id_returns_not_ok",
			ctx:    WithTenantID(context.Background(), ""),
			wantID: "",
			wantOK: false,
		},
		{
			name: "plain_string_key_does_not_collide",
			//nolint:staticcheck // deliberately using a raw string key to prove isolation
			ctx:    context.WithValue(context.Background(), "tenant_id", orgID),
			wantID: "",
			wantOK: false,
		},
		{
			name:   "overwrite_returns_latest_value",
			ctx:    WithTenantID(WithTenantID(context.Background(), "old"), orgID),
			wantID: orgID,
			wantOK: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id, ok := TenantIDFromContext(tc.ctx)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if ok && id != tc.wantID {
				t.Fatalf("id = %q, want %q", id, tc.wantID)
			}
		})
	}
}
