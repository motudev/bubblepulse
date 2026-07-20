package messaging

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/motudev/bubblepulse/internal/db/repository"
	"github.com/motudev/bubblepulse/internal/jobs"
	"github.com/motudev/bubblepulse/internal/tenancy"
)

// ErrUserNotFound is returned by Handle when the platform user has no matching
// registration in user_identities.
var ErrUserNotFound = errors.New("platform user not registered")

// ErrOrgUnresolved is returned by Handle when the platform user is known but
// cannot be mapped to an organization, so no tenant context can be established.
var ErrOrgUnresolved = errors.New("platform user has no resolvable organization")

// IncomingMessage is the normalised representation of a user update from any platform.
type IncomingMessage struct {
	Provider       string // matches user_identities.provider — the OIDC issuer URL, e.g. "https://slack.com"
	PlatformUserID string // matches user_identities.provider_id
	WorkspaceID    string // the platform's workspace/tenant ID (Slack team_id); may be empty
	Text           string
}

// dailyUpdateInserter is the transactional write side of the daily update repository.
type dailyUpdateInserter interface {
	InsertTx(ctx context.Context, tx pgx.Tx, orgID string, userID int64, text string) (int64, error)
}

// identityResolver resolves and backfills Global Directory identities.
type identityResolver interface {
	FindIdentity(ctx context.Context, provider, providerID string) (repository.IdentityRecord, error)
	UpsertIdentity(ctx context.Context, userID int64, provider, providerID, orgID string) error
}

// workspaceResolver maps platform workspace IDs to organizations.
type workspaceResolver interface {
	FindOrgByWorkspace(ctx context.Context, provider, externalID string) (string, error)
}

// jobEnqueuer is the River client subset required by MessageService.
type jobEnqueuer interface {
	InsertTx(ctx context.Context, tx pgx.Tx, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

// tenantTxRunner opens tenant-scoped transactions (satisfied by *tenancy.Runner).
type tenantTxRunner interface {
	RunTx(ctx context.Context, fn func(tx pgx.Tx) error) error
}

// MessageService handles the platform-agnostic core: Global Directory
// resolution of the sender's user and organization, then update insertion and
// NLP job enqueueing within a single tenant-scoped transaction.
type MessageService struct {
	runner     tenantTxRunner
	identities identityResolver
	workspaces workspaceResolver
	updates    dailyUpdateInserter
	queue      jobEnqueuer
}

// NewMessageService constructs a MessageService with its dependencies.
func NewMessageService(runner tenantTxRunner, identities identityResolver, workspaces workspaceResolver, updates dailyUpdateInserter, queue jobEnqueuer) *MessageService {
	return &MessageService{
		runner:     runner,
		identities: identities,
		workspaces: workspaces,
		updates:    updates,
		queue:      queue,
	}
}

// Handle resolves the platform user and organization from the Global Directory
// (no tenant context required), then within a single tenant-scoped transaction
// inserts the daily update and enqueues the NLP processing job — both carrying
// the resolved org_id.
func (s *MessageService) Handle(ctx context.Context, msg IncomingMessage) error {
	ident, err := s.identities.FindIdentity(ctx, msg.Provider, msg.PlatformUserID)
	if err != nil {
		if !errors.Is(err, repository.ErrIdentityNotFound) {
			slog.Warn("messaging: identity lookup DB error", "provider", msg.Provider, "provider_user", msg.PlatformUserID, "error", err)
		}
		return ErrUserNotFound
	}

	orgID, err := s.resolveOrg(ctx, msg, ident)
	if err != nil {
		return err
	}

	var updateID int64
	err = s.runner.RunTx(tenancy.WithTenantID(ctx, orgID), func(tx pgx.Tx) error {
		var txErr error
		updateID, txErr = s.updates.InsertTx(ctx, tx, orgID, ident.UserID, msg.Text)
		if txErr != nil {
			return fmt.Errorf("insert daily update: %w", txErr)
		}
		if _, txErr = s.queue.InsertTx(ctx, tx, jobs.NLPProcessingArgs{DailyUpdateID: updateID, OrgID: orgID}, nil); txErr != nil {
			return fmt.Errorf("enqueue NLP job: %w", txErr)
		}
		return nil
	})
	if err != nil {
		return err
	}

	slog.Debug("message handled", "provider", msg.Provider, "user_id", ident.UserID, "org_id", orgID, "update_id", updateID)
	return nil
}

// resolveOrg returns the sender's organization: from the identity itself, or —
// for legacy identities without one — from the platform workspace mapping,
// backfilling the identity for the next message.
func (s *MessageService) resolveOrg(ctx context.Context, msg IncomingMessage, ident repository.IdentityRecord) (string, error) {
	if ident.OrgID != nil {
		return *ident.OrgID, nil
	}
	if msg.WorkspaceID == "" {
		return "", ErrOrgUnresolved
	}

	orgID, err := s.workspaces.FindOrgByWorkspace(ctx, msg.Provider, msg.WorkspaceID)
	if err != nil {
		if errors.Is(err, repository.ErrWorkspaceNotFound) {
			return "", ErrOrgUnresolved
		}
		return "", fmt.Errorf("resolve workspace org: %w", err)
	}

	if err := s.identities.UpsertIdentity(ctx, ident.UserID, msg.Provider, msg.PlatformUserID, orgID); err != nil {
		slog.Warn("messaging: identity org backfill failed", "provider", msg.Provider, "provider_user", msg.PlatformUserID, "error", err)
	}
	return orgID, nil
}
