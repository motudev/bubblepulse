//go:build integration

package repository_test

import (
	"context"
	"errors"
	"testing"

	"github.com/motudev/bubblepulse/internal/db/repository"
	"github.com/motudev/bubblepulse/internal/db/testhelpers"
)

const slackProvider = "https://slack.com"

func TestWorkspaceResolution(t *testing.T) {
	env := testhelpers.Setup(t)
	ctx := context.Background()
	orgA := env.CreateOrg(t, "Org A")

	if _, _, err := env.Workspaces.ClaimWorkspace(ctx, env.Pool, orgA, slackProvider, "T111"); err != nil {
		t.Fatalf("seed workspace claim: %v", err)
	}

	tests := []struct {
		name       string
		provider   string
		externalID string
		wantOrg    string
		wantErr    error
	}{
		{"known_workspace_resolves_to_org", slackProvider, "T111", orgA, nil},
		{"unknown_workspace_is_not_found", slackProvider, "T999", "", repository.ErrWorkspaceNotFound},
		{"same_id_under_other_provider_is_not_found", "https://example-idp.test", "T111", "", repository.ErrWorkspaceNotFound},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			org, err := env.Workspaces.FindOrgByWorkspace(ctx, tc.provider, tc.externalID)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("FindOrgByWorkspace = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("FindOrgByWorkspace: %v", err)
			}
			if org != tc.wantOrg {
				t.Fatalf("resolved org = %q, want %q", org, tc.wantOrg)
			}
		})
	}
}

// TestClaimWorkspace_ConflictReturnsWinner pins the provisioning-race
// contract: the second organization claiming the same workspace must get the
// first owner's ID back with created=false, and the mapping must not change.
func TestClaimWorkspace_ConflictReturnsWinner(t *testing.T) {
	env := testhelpers.Setup(t)
	ctx := context.Background()
	winner := env.CreateOrg(t, "Winner")
	loser := env.CreateOrg(t, "Loser")

	owner, created, err := env.Workspaces.ClaimWorkspace(ctx, env.Pool, winner, slackProvider, "T222")
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}
	if !created || owner != winner {
		t.Fatalf("first claim = (owner %q, created %v), want (%q, true)", owner, created, winner)
	}

	owner, created, err = env.Workspaces.ClaimWorkspace(ctx, env.Pool, loser, slackProvider, "T222")
	if err != nil {
		t.Fatalf("conflicting claim: %v", err)
	}
	if created {
		t.Fatal("conflicting claim reported created=true; the unique constraint did not hold")
	}
	if owner != winner {
		t.Fatalf("conflicting claim returned owner %q, want the original winner %q", owner, winner)
	}

	// The stored mapping still points at the winner.
	got, err := env.Workspaces.FindOrgByWorkspace(ctx, slackProvider, "T222")
	if err != nil {
		t.Fatalf("re-resolve workspace: %v", err)
	}
	if got != winner {
		t.Fatalf("workspace now maps to %q, want %q", got, winner)
	}
}
