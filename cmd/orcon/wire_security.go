// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/wirvii/orcon/internal/adapter/codec/permissionsyaml"
	"github.com/wirvii/orcon/internal/adapter/httpapi/page"
	"github.com/wirvii/orcon/internal/adapter/kms/env"
	"github.com/wirvii/orcon/internal/adapter/kms/file"
	"github.com/wirvii/orcon/internal/adapter/postgres"
	"github.com/wirvii/orcon/internal/adapter/sqlite"
	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/authz"
	"github.com/wirvii/orcon/internal/credential/envelope"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/config"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/password"
)

// sealInterval is the pace of the audit sealer: pending entries reach a sealed block
// within this interval (SPEC-004 R-AUD-03, at most 50 ms).
const sealInterval = 50 * time.Millisecond

// identifierPurpose is the purpose the sign-in throttle's identifier key is derived for
// from the instance key (SPEC-004 R-AUTH-02: failed attempts are counted per keyed identifier).
const identifierPurpose = "orcon/identifier@1"

// pageTokenPurpose is the purpose the key of the page tokens is derived for from the instance key
// (ADR-0042); a token does not survive a change of the instance key.
const pageTokenPurpose = "orcon/page-token@1"

// persistence is what the security wiring needs from a store. Both adapters implement it
// (storetest proves that for the ports of every area).
type persistence interface { //nolint:interfacebloat // the wiring holds the store behind every port the roles use, as the storetest suite does
	identity.OrgStore
	identity.PrincipalStore
	identity.UserStore
	identity.SessionStore
	identity.AuditLog
	identity.LoginThrottleStore
	identity.OrganizationResolver
	identity.ProjectStore
	identity.IdempotencyStore
	flow.Store
	identity.MembershipStore
	authz.Store
	envelope.KeyStore
	identity.OrgKeyGapStore
	audit.Store
	engineStore
	io.Closer
}

// security is the storage, the keys and the identity services of the roles that serve
// the API. It is built once at start and closed at shutdown.
type security struct {
	store        persistence
	keyring      *envelope.Keyring
	sessions     *identity.Sessions
	evaluator    *authz.Evaluator
	sealer       *audit.Sealer
	publicOrigin string
	clock        clock.Clock
	ids          id.Generator
	pageCodec    *page.Codec
}

// openSecurity opens the configured store, loads the master key and checks it against the
// stored canary (SPEC-004 R-KEY-05), derives the sign-in identifier key from the instance
// key and wires the session service, the authorization evaluator and the audit sealer.
// A fresh store runs the first-start bootstrap, which needs the backup acknowledgement
// of the master key (keys.master.backupAcknowledgement).
func openSecurity(ctx context.Context, cfg *config.Config, logger *slog.Logger, version string) (*security, error) {
	store, err := openStore(ctx, cfg, version)
	if err != nil {
		return nil, err
	}
	sec, err := wireSecurity(ctx, cfg, logger, store)
	if err != nil {
		return nil, errors.Join(err, store.Close())
	}
	return sec, nil
}

// semverPattern matches the release versions the linker injects (MAJOR.MINOR.PATCH, optional pre-release).
var semverPattern = regexp.MustCompile(`^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$`)

// storeVersion is the binary version the store records for its migrations. A build without
// a release version (the default "dev") counts as 0.0.0, the floor every schema accepts.
func storeVersion(version string) string {
	if semverPattern.MatchString(version) {
		return version
	}
	return "0.0.0"
}

// openStore opens the store the configuration names. Migrations run at open (store.migrations.auto
// is honored by the adapters' own start-up checks).
func openStore(ctx context.Context, cfg *config.Config, version string) (persistence, error) { //nolint:ireturn // the caller uses the store through the ports it needs, whichever adapter opened it
	version = storeVersion(version)
	switch cfg.Value("store.driver").Text() {
	case "postgres":
		s, err := postgres.Open(ctx, postgresOptions(cfg, version))
		if err != nil {
			return nil, fmt.Errorf("open the postgres store: %w", err)
		}
		return s, nil
	case "sqlite":
		path := cfg.Value("store.sqlite.path").Text()
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return nil, fmt.Errorf("create the data directory: %w", err)
		}
		sync := sqlite.SynchronousFull
		if cfg.Value("store.sqlite.synchronous").Text() == "NORMAL" {
			sync = sqlite.SynchronousNormal
		}
		s, err := sqlite.Open(ctx, sqlite.Options{Path: path, BinaryVersion: version, Synchronous: sync})
		if err != nil {
			return nil, fmt.Errorf("open the sqlite store: %w", err)
		}
		return s, nil
	default:
		return nil, fmt.Errorf("store.driver %q is not supported", cfg.Value("store.driver").Text())
	}
}

// postgresOptions maps the store.postgres.* settings onto the PostgreSQL adapter.
// The pool is bounded by store.postgres.maxConnections, so the process never
// opens more connections than the operator allowed for the shared instance.
func postgresOptions(cfg *config.Config, version string) postgres.Options {
	return postgres.Options{
		DSN:           cfg.Value("store.postgres.dsn").Text(),
		BinaryVersion: version,
		MaxConns:      int32(cfg.Value("store.postgres.maxConnections").Int()), //nolint:gosec // the schema bounds maxConnections to 1..1000
	}
}

// wireSecurity loads the keys and builds the services over an open store.
func wireSecurity(ctx context.Context, cfg *config.Config, logger *slog.Logger, store persistence) (*security, error) {
	mk, err := openMasterKey(ctx, cfg, logger)
	if err != nil {
		return nil, err
	}
	kr := envelope.NewKeyring(mk, store)
	if err := checkOrBootstrapKeys(ctx, cfg, mk, kr, store, logger); err != nil {
		return nil, err
	}
	ensureOrgKeys(ctx, kr, store, logger)
	_, identKey, err := kr.ActiveSubkey(ctx, envelope.InstanceScope(), identifierPurpose)
	if err != nil {
		return nil, fmt.Errorf("derive the identifier key: %w", err)
	}
	identify, err := identity.HMACIdentifier(identKey)
	if err != nil {
		return nil, fmt.Errorf("identifier: %w", err)
	}
	clk := clock.System{}
	ids := id.NewV7Generator(time.Now)
	sessions, err := identity.NewSessions(identity.SessionDeps{
		Users: store, Sessions: store, Throttle: store, Hasher: password.Default(), Identify: identify,
		Clock: clk, IDs: ids, Audit: store,
	}, identity.SessionConfig{PublicOrigin: cfg.Value("publicUrl").Text()})
	if err != nil {
		return nil, fmt.Errorf("sessions: %w", err)
	}
	catalog, err := permissionsyaml.Parse(authz.CatalogSource())
	if err != nil {
		return nil, fmt.Errorf("load the permission catalog: %w", err)
	}
	_, pageKey, err := kr.ActiveSubkey(ctx, envelope.InstanceScope(), pageTokenPurpose)
	if err != nil {
		return nil, fmt.Errorf("derive the page token key: %w", err)
	}
	codec, err := page.NewCodec(pageKey)
	if err != nil {
		return nil, fmt.Errorf("page tokens: %w", err)
	}
	return &security{
		store: store, keyring: kr, sessions: sessions, evaluator: authz.NewEvaluator(store, catalog),
		sealer: audit.NewSealer(store, auditKeys{kr: kr}, ids, clk), publicOrigin: cfg.Value("publicUrl").Text(),
		clock: clk, ids: ids, pageCodec: codec,
	}, nil
}

// openMasterKey returns the master key provider the configuration names. The providers of
// cloud key services are declared in the configuration but not implemented yet.
func openMasterKey(ctx context.Context, cfg *config.Config, logger *slog.Logger) (envelope.MasterKey, error) { //nolint:ireturn // the provider is chosen by configuration at run time
	switch provider := cfg.Value("keys.master.provider").Text(); provider {
	case "file":
		p, err := file.New(cfg.Value("keys.master.file.path").Text())
		if err != nil {
			return nil, fmt.Errorf("master key file: %w", err)
		}
		return p, nil
	case "env":
		p, err := env.New(ctx, os.Getenv, logger)
		if err != nil {
			return nil, fmt.Errorf("master key environment: %w", err)
		}
		return p, nil
	default:
		return nil, fmt.Errorf("keys.master.provider %q is not available in this build", provider)
	}
}

// checkOrBootstrapKeys verifies the master key against the stored canary and the active keys
// (R-KEY-05, before any write). A fresh store is bootstrapped: the canary and the instance key
// are created, but only after the operator acknowledges the master key fingerprint.
func checkOrBootstrapKeys(ctx context.Context, cfg *config.Config, mk envelope.MasterKey, kr *envelope.Keyring, store persistence, logger *slog.Logger) error {
	rep, err := envelope.Verify(ctx, mk, store)
	if err != nil {
		return fmt.Errorf("verify the master key: %w", err)
	}
	if !rep.Fresh {
		logger.InfoContext(ctx, "master key verified", "active_keys", rep.ActiveKeys)
		return nil
	}
	fp, err := mk.Fingerprint(ctx)
	if err != nil {
		return fmt.Errorf("master key fingerprint: %w", err)
	}
	if err := envelope.ConfirmBackupFingerprint(fp, cfg.Value("keys.master.backupAcknowledgement").Text()); err != nil {
		return fmt.Errorf("first start needs keys.master.backupAcknowledgement set to the master key fingerprint %s: %w", fp, err)
	}
	if err := kr.InitMasterCanary(ctx); err != nil {
		return fmt.Errorf("first start: master key canary: %w", err)
	}
	if _, err := kr.CreateKey(ctx, envelope.InstanceScope()); err != nil {
		return fmt.Errorf("first start: instance key: %w", err)
	}
	logger.InfoContext(ctx, "first start: master key canary and instance key created")
	return nil
}

// ensureOrgKeys gives a key to every organization that has none (SPEC-004 R-KEY-05, SE3b). An
// organization is created together with its key, so an organization without one was created
// before that held; without a key its audit chain cannot be sealed and its credentials cannot be
// stored. It runs after the master key is verified, so every key it writes is wrapped by the
// verified master key. Each repair is logged; a failure is logged and does not stop the start,
// because it affects that organization only. Replicas that start together are safe: the second
// writer of the same key version gets [envelope.ErrKeyExists] and moves on.
func ensureOrgKeys(ctx context.Context, kr *envelope.Keyring, store identity.OrgKeyGapStore, logger *slog.Logger) {
	err := kr.EnsureOrgKeys(ctx, store, func(org identity.OrgID) {
		logger.WarnContext(ctx, "created the missing organization key", "org_id", org.String())
	})
	if err != nil {
		logger.ErrorContext(ctx, "organization key check failed", "error", err)
	}
}

// orgKeyIssuer prepares the first key of a new organization from the keyring, without storing it.
type orgKeyIssuer struct{ kr *envelope.Keyring }

// NewOrgKey implements [identity.OrgKeyIssuer].
func (i orgKeyIssuer) NewOrgKey(ctx context.Context, org identity.OrgID) (identity.OrgKeyRecord, error) {
	k, err := i.kr.NewKey(ctx, envelope.OrgScope(org))
	if err != nil {
		return identity.OrgKeyRecord{}, err
	}
	return identity.OrgKeyRecord{
		Version: k.ID.Version, Wrapped: k.Wrapped, MasterKeyReference: k.MasterKeyReference,
		Status: string(k.Status), Probe: k.Canary, CreatedAt: k.CreatedAt,
	}, nil
}

// auditKeys gives the audit sealer the MAC keys of each chain: the organization key for an
// organization chain, the instance key for the instance chain (ADR-0048 section 3).
type auditKeys struct{ kr *envelope.Keyring }

func (k auditKeys) scope(chain audit.Chain) envelope.Scope {
	if chain.IsInstance() {
		return envelope.InstanceScope()
	}
	return envelope.OrgScope(identity.OrgID(chain.Org()))
}

func (k auditKeys) ActiveMACKey(ctx context.Context, chain audit.Chain) (int64, []byte, error) {
	id, sub, err := k.kr.ActiveSubkey(ctx, k.scope(chain), audit.MACPurpose)
	if err != nil {
		return 0, nil, fmt.Errorf("active audit key: %w", err)
	}
	return int64(id.Version), sub, nil
}

func (k auditKeys) MACKey(ctx context.Context, chain audit.Chain, version int64) ([]byte, error) {
	kid := envelope.KeyID{Level: envelope.LevelInstance, Version: int(version)}
	if !chain.IsInstance() {
		kid = envelope.KeyID{Level: envelope.LevelOrg, OrgID: identity.OrgID(chain.Org()), Version: int(version)}
	}
	sub, err := k.kr.SubkeyFor(ctx, kid, audit.MACPurpose)
	if err != nil {
		return nil, fmt.Errorf("audit key version %d: %w", version, err)
	}
	return sub, nil
}

// runSealer seals the pending audit entries until ctx ends.
func (s *security) runSealer(ctx context.Context, logger *slog.Logger) {
	sealLoop(ctx, logger, defaultSealPace, s.sealer.SealOnce)
}

// close releases the store.
func (s *security) close() error {
	if s == nil {
		return nil
	}
	return s.store.Close()
}
