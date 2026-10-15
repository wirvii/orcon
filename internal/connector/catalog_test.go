// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package connector_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/connector"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
)

// world is a registry over in-memory stores with two organizations.
type world struct {
	t        *testing.T
	store    *memStore
	policy   *blockingPolicy
	registry *connector.Registry
	orgA     identity.OrgID
	orgB     identity.OrgID
	project  identity.ProjectID
	admin    identity.PrincipalID
}

func newWorld(t *testing.T) *world {
	t.Helper()
	store := &memStore{}
	policy := &blockingPolicy{}
	ids := id.NewSequence()
	clk := clock.NewFake(time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC))
	return &world{
		t: t, store: store, policy: policy,
		registry: connector.NewRegistry(store, store, policy, clk, ids),
		orgA:     identity.OrgID(ids.New()), orgB: identity.OrgID(ids.New()),
		project: identity.ProjectID(ids.New()), admin: identity.PrincipalID(ids.New()),
	}
}

// install builds and installs the reference connector at a version.
func (w *world) install(version string) connector.PackageRecord {
	w.t.Helper()
	text := strings.ReplaceAll(referenceManifest, `1.2.0`, version)
	p := buildOK(w.t, referenceFiles(text))
	rec, err := w.registry.Install(w.t.Context(), p.Archive(), w.admin)
	if err != nil {
		w.t.Fatalf("Install(%s) failed: %v", version, err)
	}
	return rec
}

func (w *world) grant(org identity.OrgID, project identity.ProjectID, rec connector.PackageRecord) {
	w.t.Helper()
	if _, err := w.registry.Grant(w.t.Context(), scopeFor(org), project, rec, w.admin); err != nil {
		w.t.Fatalf("Grant() failed: %v", err)
	}
}

// packageNode builds a flow node of a type with an optional pin; operation, when
// not empty, is written as the literal "operation" parameter.
func packageNode(t *testing.T, nodeType string, pin *flow.ConnectorPin, operation string) *flow.Node {
	t.Helper()
	params := `{}`
	if operation != "" {
		params = fmt.Sprintf(`{ "operation": %q }`, operation)
	}
	return &flow.Node{ID: "n1", Type: nodeType, Parameters: value(t, params), Connector: pin}
}

func TestInstallRecordsThePackage(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	rec := w.install("1.2.0")
	if rec.ConnectorID != "acme.shop" || rec.Version != "1.2.0" || rec.Kind != connector.KindDeclarative || rec.Status != connector.StatusInstalled {
		t.Errorf("record = %+v", rec)
	}
	if rec.InstalledByPrincipalID != w.admin || rec.InstalledAt.IsZero() || !strings.HasPrefix(rec.Digest, connector.DigestPrefix) {
		t.Errorf("record = %+v", rec)
	}
	if _, err := connector.Parse(rec.Manifest, connector.Options{}); err != nil {
		t.Errorf("the stored manifest does not read back: %v", err)
	}

	again := w.install("1.2.0")
	if again.ID != rec.ID || len(w.store.packages) != 1 {
		t.Errorf("installing the same archive again created a second record: %+v", again)
	}
}

func TestInstallRefusesAChangedVersion(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.install("1.2.0")
	files := referenceFiles(referenceManifest)
	files[2].Data = []byte("a different license text\n")
	changed := buildOK(t, files)
	_, err := w.registry.Install(t.Context(), changed.Archive(), w.admin)
	if !errors.Is(err, connector.ErrDigestConflict) {
		t.Errorf("Install() of different content for an installed version = %v, want ErrDigestConflict", err)
	}
}

func TestInstallRejectsAnInvalidArchive(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	if _, err := w.registry.Install(t.Context(), []byte("garbage"), w.admin); !errors.Is(err, connector.ErrArchive) {
		t.Errorf("Install(garbage) = %v, want ErrArchive", err)
	}
	if len(w.store.packages) != 0 {
		t.Error("an invalid archive left a record")
	}
}

func TestRegisterBuiltinAllowsTheReservedNamespace(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	text := strings.ReplaceAll(strings.ReplaceAll(referenceManifest, `"id": "acme.shop"`, `"id": "orcon.shop"`), `acme.shop.`, `orcon.shop.`)
	rec, err := w.registry.RegisterBuiltin(t.Context(), []byte(text))
	if err != nil {
		t.Fatalf("RegisterBuiltin() failed: %v", err)
	}
	if rec.ConnectorID != "orcon.shop" || rec.InstalledByPrincipalID != (identity.PrincipalID{}) {
		t.Errorf("record = %+v", rec)
	}
	if _, err := w.registry.RegisterBuiltin(t.Context(), []byte(text)); err != nil {
		t.Errorf("registering the same built-in connector twice must be idempotent: %v", err)
	}
}

func TestGrantRecordsThePermissionsDigest(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	rec := w.install("1.2.0")
	w.grant(w.orgA, identity.ProjectID{}, rec)
	g, err := w.store.FindGrant(t.Context(), scopeFor(w.orgA), rec.ID, w.project)
	if err != nil {
		t.Fatal(err)
	}
	m := readOK(t, strings.ReplaceAll(referenceManifest, `1.2.0`, "1.2.0"))
	want, _ := m.PermissionsDigest()
	if g.PermissionsDigest != want || g.GrantedByPrincipalID != w.admin {
		t.Errorf("grant = %+v, want permissions digest %s", g, want)
	}
	if _, err := w.registry.Grant(t.Context(), scopeFor(w.orgA), identity.ProjectID{}, rec, w.admin); !errors.Is(err, connector.ErrConflict) {
		t.Errorf("a second organization-wide grant = %v, want ErrConflict", err)
	}
}

func TestLookupDescribesTheNewestVersion(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.install("1.2.0")
	w.install("1.10.0")
	w.install("1.9.0")
	cat := w.registry.Catalog(scopeFor(w.orgA), w.project, nil)

	nt, ok := cat.Lookup(t.Context(), "acme.shop.order@1")
	if !ok || nt.Purity != flow.PurityEffect || nt.Contract == nil || nt.Contract.Level != flow.SafetyUnsafe {
		t.Errorf("Lookup(order) = %+v, %v; a node with several operations must show its riskiest one", nt, ok)
	}
	trigger, ok := cat.Lookup(t.Context(), "acme.shop.orderCreated@1")
	if !ok || trigger.Purity != flow.PurityRead || trigger.Contract != nil {
		t.Errorf("Lookup(trigger) = %+v, %v", trigger, ok)
	}
	for _, unknown := range []string{"acme.shop.missing@1", "orcon.http@1", "other.shop.order@1"} {
		if _, ok := cat.Lookup(t.Context(), unknown); ok {
			t.Errorf("Lookup(%s) found a type nobody defines", unknown)
		}
	}
}

func TestResolveNodeUsesThePinnedVersion(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	old := w.install("1.2.0")
	// Version 2.0.0 weakens order.create to UNSAFE; a node pinned to 1.2.0 keeps the old contract.
	weaker := strings.Replace(strings.ReplaceAll(referenceManifest, `1.2.0`, "2.0.0"),
		`"effect": "WRITE", "level": "IDEMPOTENCY_KEY",
            "idempotency": { "mode": "PROVIDER_KEY", "location": { "in": "HEADER", "name": "Idempotency-Key" }, "ttl": "PT24H", "scope": "CREDENTIAL" },
            "onUncertain": "RESEND_WITH_KEY", "retrySafe": true,`,
		`"effect": "WRITE", "level": "UNSAFE", "onUncertain": "SUSPEND", "retrySafe": false,`, 1)
	newer, err := w.registry.Install(t.Context(), buildOK(t, referenceFiles(weaker)).Archive(), w.admin)
	if err != nil {
		t.Fatal(err)
	}
	w.grant(w.orgA, identity.ProjectID{}, old)
	w.grant(w.orgA, identity.ProjectID{}, newer)
	cat := w.registry.Catalog(scopeFor(w.orgA), w.project, nil)

	res := cat.ResolveNode(t.Context(), packageNode(t, "acme.shop.order@1", &flow.ConnectorPin{Version: "1.2.0", Digest: old.Digest}, "order.create"))
	if !res.Known || len(res.Problems) != 0 || res.Type.Contract.Level != flow.SafetyIdempotencyKey {
		t.Errorf("pinned to 1.2.0: %+v", res)
	}
	res = cat.ResolveNode(t.Context(), packageNode(t, "acme.shop.order@1", &flow.ConnectorPin{Version: "2.0.0", Digest: newer.Digest}, "order.create"))
	if !res.Known || len(res.Problems) != 0 || res.Type.Contract.Level != flow.SafetyUnsafe {
		t.Errorf("pinned to 2.0.0: %+v", res)
	}
	res = cat.ResolveNode(t.Context(), packageNode(t, "acme.shop.order@1", &flow.ConnectorPin{Version: "1.2.0", Digest: old.Digest}, "order.nope"))
	if !res.Known || res.Selection != flow.SelectionUnknown {
		t.Errorf("an operation the version does not have: %+v", res)
	}
	res = cat.ResolveNode(t.Context(), packageNode(t, "orcon.http@1", nil, ""))
	if res.Known || len(res.Problems) != 0 || res.Selection != flow.SelectionOK {
		t.Errorf("a type no package can own must come back unknown and without pin problems: %+v", res)
	}
}

func TestResolveNodeNamesEveryPinProblem(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	rec := w.install("1.2.0")
	w.policy.block("acme.shop", "1.2.0", "revoked by the test")
	cat := w.registry.Catalog(scopeFor(w.orgA), w.project, nil)
	wrongDigest := "sha256:" + strings.Repeat("0", 64)

	res := cat.ResolveNode(t.Context(), packageNode(t, "acme.shop.order@1", &flow.ConnectorPin{Version: "1.2.0", Digest: wrongDigest}, ""))
	joined := strings.Join(res.Problems, "\n")
	for _, want := range []string{"digest " + wrongDigest, rec.Digest, "not granted", "policy blocks", "revoked by the test"} {
		if !strings.Contains(joined, want) {
			t.Errorf("problems do not mention %q:\n%s", want, joined)
		}
	}
	res = cat.ResolveNode(t.Context(), packageNode(t, "acme.shop.order@1", &flow.ConnectorPin{Version: "9.9.9", Digest: rec.Digest}, ""))
	if res.Known || len(res.Problems) != 1 || !strings.Contains(res.Problems[0], "version 9.9.9, which is not installed") {
		t.Errorf("a version that is not installed: %+v", res)
	}
	res = cat.ResolveNode(t.Context(), packageNode(t, "acme.shop.order@1", nil, ""))
	if !res.Known || len(res.Problems) != 1 || !strings.Contains(res.Problems[0], "no connector pin") {
		t.Errorf("a node without a pin: %+v", res)
	}
}

func TestGrantsAreScopedToTheOrganizationAndProject(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	rec := w.install("1.2.0")
	pin := &flow.ConnectorPin{Version: "1.2.0", Digest: rec.Digest}
	w.grant(w.orgB, identity.ProjectID{}, rec)
	otherProject := identity.ProjectID(id.NewSequence().New())
	w.grant(w.orgA, otherProject, rec)

	problems := func(org identity.OrgID, project identity.ProjectID) int {
		return len(w.registry.Catalog(scopeFor(org), project, nil).ResolveNode(t.Context(), packageNode(t, "acme.shop.order@1", pin, "")).Problems)
	}
	if problems(w.orgA, w.project) == 0 {
		t.Error("another organization's grant, and another project's, let this project use the package")
	}
	if problems(w.orgB, w.project) != 0 {
		t.Error("an organization-wide grant does not cover its projects")
	}
	if problems(w.orgA, otherProject) != 0 {
		t.Error("a project grant does not cover its own project")
	}
}

func TestCompareVersions(t *testing.T) {
	t.Parallel()
	ordered := []string{
		"0.9.0", "1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta", "1.0.0-beta.2", "1.0.0-beta.11", "1.0.0-rc.1",
		"1.0.0", "1.0.1", "1.2.0", "1.9.0", "1.10.0", "2.0.0",
	}
	for i, a := range ordered {
		for j, b := range ordered {
			got := connector.CompareVersions(a, b)
			want := 0
			switch {
			case i < j:
				want = -1
			case i > j:
				want = 1
			}
			if got != want {
				t.Errorf("CompareVersions(%s, %s) = %d, want %d", a, b, got, want)
			}
		}
	}
	if connector.CompareVersions("1.0.0+build.1", "1.0.0+build.2") != 0 {
		t.Error("build metadata must not change precedence")
	}
	if connector.CompareVersions("nonsense", "0.0.1") >= 0 || connector.CompareVersions("0.0.1", "nonsense") <= 0 {
		t.Error("a text that is not a version must sort below every valid version")
	}
}
