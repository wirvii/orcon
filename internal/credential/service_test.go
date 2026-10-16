// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package credential

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/credential/envelope"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
	"github.com/wirvii/orcon/internal/platform/orgscope/issue"
)

// memStore is an in-memory [Store] with the scoping rules of the adapters: every row is found only under its own
// organization, and a share needs a project of that organization.
type memStore struct {
	mu       sync.Mutex
	projects map[identity.OrgID]map[identity.ProjectID]bool
	creds    map[id.UUID]Record
	versions map[id.UUID]map[int]Version
	shares   map[string]Share
	entries  []audit.Entry
}

// record keeps the audit entry a mutator was given, so a test can check what the store was asked to write.
func (m *memStore) record(e audit.Entry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, e)
}

func newMemStore() *memStore {
	return &memStore{
		projects: map[identity.OrgID]map[identity.ProjectID]bool{},
		creds:    map[id.UUID]Record{}, versions: map[id.UUID]map[int]Version{}, shares: map[string]Share{},
	}
}

func shareKey(cred id.UUID, project identity.ProjectID) string {
	return cred.String() + "/" + id.UUID(project).String()
}

func (m *memStore) addProject(org identity.OrgID, p identity.ProjectID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.projects[org] == nil {
		m.projects[org] = map[identity.ProjectID]bool{}
	}
	m.projects[org][p] = true
}

func (m *memStore) owned(org orgscope.Org, cred id.UUID) (Record, bool) {
	rec, ok := m.creds[cred]
	return rec, ok && !org.IsZero() && rec.OrgID == identity.OrgIDOf(org)
}

func (m *memStore) CreateCredential(_ context.Context, org orgscope.Org, rec Record, first Version, entry audit.Entry) error {
	m.record(entry)
	m.mu.Lock()
	defer m.mu.Unlock()
	if org.IsZero() || rec.OrgID != identity.OrgIDOf(org) {
		return ErrNotFound
	}
	for k := range m.creds {
		other := m.creds[k]
		if other.OrgID == rec.OrgID && other.ProjectID == rec.ProjectID && other.Name == rec.Name {
			return ErrConflict
		}
	}
	m.creds[rec.ID] = rec
	m.versions[rec.ID] = map[int]Version{first.Number: first}
	return nil
}

func (m *memStore) GetCredential(_ context.Context, org orgscope.Org, cred id.UUID) (Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.owned(org, cred)
	if !ok {
		return Record{}, ErrNotFound
	}
	return rec, nil
}

func (m *memStore) ListCredentials(_ context.Context, org orgscope.Org, project identity.ProjectID) ([]Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Record
	for k := range m.creds {
		rec := m.creds[k]
		if rec.OrgID != identity.OrgIDOf(org) || org.IsZero() {
			continue
		}
		if _, shared := m.shares[shareKey(rec.ID, project)]; shared || rec.ProjectID == project {
			out = append(out, rec)
		}
	}
	return out, nil
}

func (m *memStore) GetCredentialVersion(_ context.Context, org orgscope.Org, cred id.UUID, number int) (Version, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.owned(org, cred); !ok {
		return Version{}, ErrNotFound
	}
	v, ok := m.versions[cred][number]
	if !ok {
		return Version{}, ErrNotFound
	}
	return v, nil
}

func (m *memStore) AddVersion(_ context.Context, org orgscope.Org, cred id.UUID, expected int, v Version, status Status, at time.Time, entry audit.Entry) error {
	m.record(entry)
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.owned(org, cred)
	if !ok {
		return ErrNotFound
	}
	if rec.CurrentVersion != expected {
		return ErrConflict
	}
	rec.CurrentVersion = v.Number
	rec.Status = status
	rec.UpdatedAt = at
	rec.SecretUpdatedAt = at
	rec.SecretUpdatedBy = v.CreatedBy
	m.creds[cred] = rec
	m.versions[cred][v.Number] = v
	return nil
}

func (m *memStore) UpdateCredential(_ context.Context, org orgscope.Org, rec Record, expected time.Time, entry audit.Entry) error {
	m.record(entry)
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, ok := m.owned(org, rec.ID)
	if !ok {
		return ErrNotFound
	}
	if !cur.UpdatedAt.Equal(expected) {
		return ErrConflict
	}
	for k := range m.creds {
		other := m.creds[k]
		if other.ID != rec.ID && other.OrgID == cur.OrgID && other.ProjectID == cur.ProjectID && other.Name == rec.Name {
			return ErrConflict
		}
	}
	cur.Name, cur.AllowedHosts, cur.UpdatedAt = rec.Name, rec.AllowedHosts, rec.UpdatedAt
	m.creds[rec.ID] = cur
	return nil
}

func (m *memStore) DeleteCredential(_ context.Context, org orgscope.Org, project identity.ProjectID, cred id.UUID, expected time.Time, entry audit.Entry) error {
	m.record(entry)
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.owned(org, cred)
	if !ok || rec.ProjectID != project {
		return ErrNotFound
	}
	if !rec.UpdatedAt.Equal(expected) {
		return ErrConflict
	}
	delete(m.creds, cred)
	delete(m.versions, cred)
	for k, s := range m.shares {
		if s.CredentialID == cred {
			delete(m.shares, k)
		}
	}
	return nil
}

func (m *memStore) GetShare(_ context.Context, org orgscope.Org, cred id.UUID, project identity.ProjectID) (Share, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.owned(org, cred); !ok {
		return Share{}, ErrNotFound
	}
	s, ok := m.shares[shareKey(cred, project)]
	if !ok {
		return Share{}, ErrNotFound
	}
	return s, nil
}

func (m *memStore) CreateShare(_ context.Context, org orgscope.Org, s Share, entry audit.Entry) error {
	m.record(entry)
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.owned(org, s.CredentialID); !ok {
		return ErrNotFound
	}
	if !m.projects[identity.OrgIDOf(org)][s.ProjectID] {
		return ErrNotFound // the composite foreign key of the adapters: the project is not in this organization
	}
	k := shareKey(s.CredentialID, s.ProjectID)
	if _, dup := m.shares[k]; dup {
		return ErrConflict
	}
	m.shares[k] = s
	return nil
}

func (m *memStore) DeleteShare(_ context.Context, org orgscope.Org, cred id.UUID, project identity.ProjectID, entry audit.Entry) error {
	m.record(entry)
	m.mu.Lock()
	defer m.mu.Unlock()
	k := shareKey(cred, project)
	if _, ok := m.shares[k]; !ok {
		return ErrNotFound
	}
	if _, ok := m.owned(org, cred); !ok {
		return ErrNotFound
	}
	delete(m.shares, k)
	return nil
}

// fakeKeys stands in for the envelope keyring: it wraps data keys in memory and unwraps them only under the key
// version they were wrapped with. It keeps the key material and builds a fresh [envelope.Key] on every call, because
// the service destroys the keys it is handed.
type fakeKeys struct {
	mu      sync.Mutex
	wrapped map[string][]byte
	n       int
}

func (f *fakeKeys) GenerateDataKey(_ context.Context, scope envelope.Scope) (envelope.DataKey, error) {
	material := make([]byte, 32)
	if _, err := rand.Read(material); err != nil {
		return envelope.DataKey{}, err
	}
	key, err := envelope.NewKey(material)
	if err != nil {
		return envelope.DataKey{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.n++
	w := []byte(fmt.Sprintf("wrapped-%d", f.n))
	f.wrapped[string(w)] = material
	return envelope.DataKey{ID: envelope.KeyID{Level: envelope.LevelOrg, OrgID: scope.OrgID, Version: 1}, Key: key, Wrapped: w}, nil
}

func (f *fakeKeys) UnwrapDataKey(_ context.Context, kid envelope.KeyID, wrapped []byte) (envelope.Key, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	material, ok := f.wrapped[string(wrapped)]
	if !ok || kid.Version != 1 {
		return envelope.Key{}, envelope.ErrKeyUnavailable
	}
	return envelope.NewKey(material)
}

// fixture is one service over the fakes with two organizations, A and B, and a project in each plus a second
// project in A.
type fixture struct {
	t      *testing.T
	svc    *Service
	store  *memStore
	orgA   orgscope.Org
	orgB   orgscope.Org
	projA1 identity.ProjectID
	projA2 identity.ProjectID
	projB1 identity.ProjectID
	actor  audit.Actor
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ids := id.NewSequence()
	a, err := issue.New(ids.New())
	if err != nil {
		t.Fatal(err)
	}
	b, err := issue.New(ids.New())
	if err != nil {
		t.Fatal(err)
	}
	st := newMemStore()
	f := &fixture{
		t: t, store: st, orgA: a, orgB: b,
		projA1: identity.ProjectID(ids.New()), projA2: identity.ProjectID(ids.New()), projB1: identity.ProjectID(ids.New()),
		actor: audit.Actor{PrincipalID: ids.New(), Kind: audit.ActorUser},
	}
	st.addProject(identity.OrgIDOf(a), f.projA1)
	st.addProject(identity.OrgIDOf(a), f.projA2)
	st.addProject(identity.OrgIDOf(b), f.projB1)
	clk := clock.NewFake(time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC))
	svc, err := New(Config{Store: st, Keys: &fakeKeys{wrapped: map[string][]byte{}}, Clock: clk, IDs: ids})
	if err != nil {
		t.Fatal(err)
	}
	f.svc = svc
	return f
}

// basicInput is a valid credential input with one public and one secret field.
func basicInput(secret string) Input {
	return Input{
		Name: "WhatsApp token", ConnectorReference: "whatsapp", AllowedHosts: []string{"graph.facebook.com"},
		Public: map[string]string{"phoneNumberId": "1234567890"},
		Secret: map[string]string{"accessToken": secret},
	}
}

func (f *fixture) mustCreate(in Input) Summary {
	f.t.Helper()
	s, err := f.svc.Create(f.t.Context(), f.orgA, f.projA1, in, f.actor)
	if err != nil {
		f.t.Fatalf("create: %v", err)
	}
	return s
}

// TestSecretFieldsNeverReturned (E-CRED-1, R-CRED-01): no value that a reader can get back, through a summary, a
// listing, a formatted value or a stored row in clear, contains the secret.
func TestSecretFieldsNeverReturned(t *testing.T) {
	f := newFixture(t)
	const sentinelValue = "EAAG-super-secret-value-7781"
	created := f.mustCreate(basicInput(sentinelValue))

	got, err := f.svc.Get(f.t.Context(), f.orgA, f.projA1, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	list, err := f.svc.List(f.t.Context(), f.orgA, f.projA1)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	dumps := []string{fmt.Sprintf("%+v %#v", created, created), fmt.Sprintf("%+v", got), fmt.Sprintf("%+v", list)}
	for _, d := range dumps {
		if strings.Contains(d, sentinelValue) {
			t.Errorf("a summary contains the secret: %s", d)
		}
	}
	if !got.Secret.Set || got.Public["phoneNumberId"] != "1234567890" {
		t.Errorf("get = %+v; want the sentinelValue state set and the public field readable", got.Secret)
	}

	ver, err := f.store.GetCredentialVersion(f.t.Context(), f.orgA, created.ID, 1)
	if err != nil {
		t.Fatalf("stored version: %v", err)
	}
	if bytes.Contains(ver.SecretCiphertext, []byte(sentinelValue)) || bytes.Contains(ver.DataKeyWrapped, []byte(sentinelValue)) {
		t.Error("the stored ciphertext contains the sentinelValue in clear")
	}

	res, err := f.svc.Resolve(f.t.Context(), f.orgA, f.projA1, created.ID)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if v, ok := res.Secret("accessToken"); !ok || v != sentinelValue {
		t.Errorf("resolve did not return the sentinelValue to the dispatch")
	}
	if d := fmt.Sprintf("%v %+v %#v", res, res, res); strings.Contains(d, sentinelValue) {
		t.Errorf("the resolved credential prints its secret: %s", d)
	}

	_, err = f.svc.Create(f.t.Context(), f.orgA, f.projA1, Input{
		Name: "bad", ConnectorReference: "whatsapp", AllowedHosts: []string{"https://x.example.com"},
		Secret: map[string]string{"k": sentinelValue},
	}, f.actor)
	if err == nil || strings.Contains(err.Error(), sentinelValue) {
		t.Errorf("an invalid input error = %v; want an error that does not carry the value", err)
	}
}

// TestReplaceCreatesVersion (E-CRED-1, R-CRED-02): replacing the secret stores a new version, the dispatch uses the
// current one and the previous version is kept.
func TestReplaceCreatesVersion(t *testing.T) {
	f := newFixture(t)
	created := f.mustCreate(basicInput("first-value"))

	next := basicInput("second-value")
	after, err := f.svc.Replace(f.t.Context(), f.orgA, f.projA1, created.ID, next, f.actor)
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if after.CurrentVersion != 2 {
		t.Errorf("current version = %d after a replace; want 2", after.CurrentVersion)
	}
	res, err := f.svc.Resolve(f.t.Context(), f.orgA, f.projA1, created.ID)
	if err != nil {
		t.Fatalf("resolve after replace: %v", err)
	}
	if res.Version != 2 {
		t.Errorf("dispatch resolved version %d; want 2", res.Version)
	}
	if v, _ := res.Secret("accessToken"); v != "second-value" {
		t.Error("the dispatch did not see the replaced secret")
	}
	old, err := f.store.GetCredentialVersion(f.t.Context(), f.orgA, created.ID, 1)
	if err != nil || old.Number != 1 {
		t.Fatalf("version 1 after a replace = %v, %v; want it kept", old.Number, err)
	}
	// A replacement that is not the owner's is refused and changes nothing.
	if _, err := f.svc.Replace(f.t.Context(), f.orgA, f.projA2, created.ID, next, f.actor); !errors.Is(err, ErrNotFound) {
		t.Errorf("replace by an unrelated project = %v; want ErrNotFound", err)
	}
	if _, err := f.svc.Replace(f.t.Context(), f.orgA, f.projA1, created.ID, Input{Public: map[string]string{"k": "v"}}, f.actor); !errors.Is(err, ErrInvalid) {
		t.Errorf("replace without a secret = %v; want ErrInvalid", err)
	}
}

// TestInstanceAdminCannotRead (E-CRED-1, R-ISO-01): a reader without a scope of the organization, or without a
// relation to the credential, gets not-found and no metadata.
func TestInstanceAdminCannotRead(t *testing.T) {
	f := newFixture(t)
	created := f.mustCreate(basicInput("admin-probe"))

	var none orgscope.Org
	if _, err := f.svc.Get(f.t.Context(), none, f.projA1, created.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("get with no organization scope = %v; want ErrNotFound", err)
	}
	if _, err := f.svc.Get(f.t.Context(), f.orgB, f.projB1, created.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("get from another organization = %v; want ErrNotFound", err)
	}
	if _, err := f.svc.Resolve(f.t.Context(), f.orgB, f.projB1, created.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("resolve from another organization = %v; want ErrNotFound", err)
	}
	if _, err := f.svc.Get(f.t.Context(), f.orgA, f.projA2, created.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("get by a project without a share = %v; want ErrNotFound", err)
	}
}

// TestHostBindingBlocksOtherHosts (E-CRED-2, R-CRED-03): the credential is applied only to its allowed hosts; a
// wildcard entry matches exactly one extra label.
func TestHostBindingBlocksOtherHosts(t *testing.T) {
	f := newFixture(t)
	in := basicInput("bound")
	in.AllowedHosts = []string{"api.example.com", "*.hooks.example.com"}
	created := f.mustCreate(in)
	res, err := f.svc.Resolve(f.t.Context(), f.orgA, f.projA1, created.ID)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	allowed := []string{"api.example.com", "API.example.com.", "a.hooks.example.com"}
	for _, h := range allowed {
		if err := res.CheckHost(h); err != nil {
			t.Errorf("CheckHost(%q) = %v; want allowed", h, err)
		}
	}
	denied := []string{"evil.example.com", "x.api.example.com", "hooks.example.com", "a.b.hooks.example.com", "", "api.example.com.evil.net"}
	for _, h := range denied {
		if err := res.CheckHost(h); !errors.Is(err, ErrHostNotAllowed) {
			t.Errorf("CheckHost(%q) = %v; want ErrHostNotAllowed", h, err)
		}
	}
	for _, bad := range []string{"https://api.example.com", "*", "localhost:8080/x", "bad_name.example.com"} {
		if _, err := f.svc.Create(f.t.Context(), f.orgA, f.projA1, withHosts(basicInput("v"), bad), f.actor); !errors.Is(err, ErrInvalid) {
			t.Errorf("an allowed host %q = %v; want ErrInvalid", bad, err)
		}
	}
}

func withHosts(in Input, hosts ...string) Input {
	in.Name = in.Name + " " + strings.Join(hosts, "")
	in.AllowedHosts = hosts
	return in
}

// TestShareUseWithoutRead (E-CRED-2, R-AZ-07): a shared project uses the credential at dispatch but cannot read
// its secret through the summary, cannot change it and cannot share it on.
func TestShareUseWithoutRead(t *testing.T) {
	f := newFixture(t)
	created := f.mustCreate(basicInput("shared-value"))
	if err := f.svc.Share(f.t.Context(), f.orgA, f.projA1, created.ID, f.projA2, f.actor); err != nil {
		t.Fatalf("share: %v", err)
	}

	res, err := f.svc.Resolve(f.t.Context(), f.orgA, f.projA2, created.ID)
	if err != nil {
		t.Fatalf("the shared project cannot use the credential: %v", err)
	}
	if v, _ := res.Secret("accessToken"); v != "shared-value" {
		t.Error("the shared project's dispatch did not receive the secret")
	}
	seen, err := f.svc.Get(f.t.Context(), f.orgA, f.projA2, created.ID)
	if err != nil {
		t.Fatalf("get by the shared project: %v", err)
	}
	if seen.Owned || !seen.Secret.Set {
		t.Errorf("shared view = owned %v, secret %+v; want not owned, secret set", seen.Owned, seen.Secret)
	}
	if _, err := f.svc.Replace(f.t.Context(), f.orgA, f.projA2, created.ID, basicInput("x"), f.actor); !errors.Is(err, ErrNotOwner) {
		t.Errorf("replace by the shared project = %v; want ErrNotOwner", err)
	}
	if err := f.svc.Delete(f.t.Context(), f.orgA, f.projA2, created.ID, time.Time{}, f.actor); !errors.Is(err, ErrNotOwner) {
		t.Errorf("delete by the shared project = %v; want ErrNotOwner", err)
	}
	if err := f.svc.Share(f.t.Context(), f.orgA, f.projA2, created.ID, f.projA1, f.actor); !errors.Is(err, ErrNotOwner) {
		t.Errorf("share by the shared project = %v; want ErrNotOwner", err)
	}
}

// TestUnshareStopsNextDispatch (E-CRED-2, R-AZ-07): removing the share makes the next dispatch of the target fail
// with an explicit error, and the owner keeps working.
func TestUnshareStopsNextDispatch(t *testing.T) {
	f := newFixture(t)
	created := f.mustCreate(basicInput("revocable"))
	if err := f.svc.Share(f.t.Context(), f.orgA, f.projA1, created.ID, f.projA2, f.actor); err != nil {
		t.Fatalf("share: %v", err)
	}
	if _, err := f.svc.Resolve(f.t.Context(), f.orgA, f.projA2, created.ID); err != nil {
		t.Fatalf("before the unshare: %v", err)
	}
	if err := f.svc.Unshare(f.t.Context(), f.orgA, f.projA1, created.ID, f.projA2, f.actor); err != nil {
		t.Fatalf("unshare: %v", err)
	}
	if _, err := f.svc.Resolve(f.t.Context(), f.orgA, f.projA2, created.ID); !errors.Is(err, ErrNotUsable) {
		t.Errorf("the next dispatch after the unshare = %v; want ErrNotUsable", err)
	}
	if _, err := f.svc.Resolve(f.t.Context(), f.orgA, f.projA1, created.ID); err != nil {
		t.Errorf("the owner's dispatch after the unshare: %v", err)
	}
}

// TestShareNeverCrossOrg (E-CRED-2, R-AZ-07): a credential is never shared with a project of another organization,
// and an organization B reader cannot share a credential of organization A.
func TestShareNeverCrossOrg(t *testing.T) {
	f := newFixture(t)
	created := f.mustCreate(basicInput("tenant-a"))
	if err := f.svc.Share(f.t.Context(), f.orgA, f.projA1, created.ID, f.projB1, f.actor); !errors.Is(err, ErrNotFound) {
		t.Errorf("share to a project of another organization = %v; want ErrNotFound", err)
	}
	if err := f.svc.Share(f.t.Context(), f.orgB, f.projB1, created.ID, f.projA2, f.actor); !errors.Is(err, ErrNotFound) {
		t.Errorf("share from another organization = %v; want ErrNotFound", err)
	}
	if _, err := f.svc.Resolve(f.t.Context(), f.orgB, f.projB1, created.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("resolve from another organization = %v; want ErrNotFound", err)
	}
}

// TestCredentialLifecycle covers creation rules, the update of settings, the status after a replace and the delete.
func TestCredentialLifecycle(t *testing.T) {
	f := newFixture(t)
	first := f.mustCreate(basicInput("one"))
	if _, err := f.svc.Create(f.t.Context(), f.orgA, f.projA1, basicInput("two"), f.actor); !errors.Is(err, ErrConflict) {
		t.Errorf("the same name twice = %v; want ErrConflict", err)
	}
	upd, err := f.svc.Update(f.t.Context(), f.orgA, f.projA1, first.ID, Settings{Name: "Renamed", AllowedHosts: []string{"graph.facebook.com", "graph.facebook.com"}}, time.Time{}, f.actor)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if upd.Name != "Renamed" || len(upd.AllowedHosts) != 1 {
		t.Errorf("update = name %q hosts %v; want the new name and one host", upd.Name, upd.AllowedHosts)
	}
	if _, err := f.svc.Update(f.t.Context(), f.orgA, f.projA2, first.ID, Settings{Name: "x", AllowedHosts: []string{"a.example.com"}}, time.Time{}, f.actor); !errors.Is(err, ErrNotFound) {
		t.Errorf("update by an unrelated project = %v; want ErrNotFound", err)
	}
	if err := f.svc.Delete(f.t.Context(), f.orgA, f.projA1, first.ID, time.Time{}, f.actor); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := f.svc.Resolve(f.t.Context(), f.orgA, f.projA1, first.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("resolve after delete = %v; want ErrNotFound", err)
	}
}

// TestNeedsSecretBecomesOKOnReplace checks that a credential waiting for its value is refused at dispatch until a
// replacement supplies it.
func TestNeedsSecretBecomesOKOnReplace(t *testing.T) {
	f := newFixture(t)
	created := f.mustCreate(basicInput("pending"))
	rec := f.store.creds[created.ID]
	rec.Status = StatusNeedsSecret
	f.store.creds[created.ID] = rec
	if _, err := f.svc.Resolve(f.t.Context(), f.orgA, f.projA1, created.ID); !errors.Is(err, ErrNotUsable) {
		t.Errorf("resolve while NEEDS_SECRET = %v; want ErrNotUsable", err)
	}
	if _, err := f.svc.Replace(f.t.Context(), f.orgA, f.projA1, created.ID, basicInput("supplied"), f.actor); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if _, err := f.svc.Resolve(f.t.Context(), f.orgA, f.projA1, created.ID); err != nil {
		t.Errorf("resolve after the replace: %v", err)
	}
}

// TestNewRequiresCollaborators checks the constructor refuses a missing port.
func TestNewRequiresCollaborators(t *testing.T) {
	if _, err := New(Config{}); err == nil {
		t.Error("New with no collaborators succeeded")
	}
}

// TestMutationsAuditEveryChangeWithoutValues (SPEC-004 R-AUD-02, R-AUD-08, Annex B): each change hands the store
// exactly one entry of the right action, by the caller, in the organization's chain, and no entry carries a secret
// value or a field name that a secret could be read from.
func TestMutationsAuditEveryChangeWithoutValues(t *testing.T) {
	f := newFixture(t)
	const secret = "probe-secret-value-7f3a"
	created := f.mustCreate(basicInput(secret))
	if _, err := f.svc.Replace(f.t.Context(), f.orgA, f.projA1, created.ID, Input{Secret: map[string]string{"accessToken": "rotated-" + secret}}, f.actor); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if _, err := f.svc.Update(f.t.Context(), f.orgA, f.projA1, created.ID, Settings{Name: "Renamed", AllowedHosts: created.AllowedHosts}, time.Time{}, f.actor); err != nil {
		t.Fatalf("update: %v", err)
	}
	if err := f.svc.Share(f.t.Context(), f.orgA, f.projA1, created.ID, f.projA2, f.actor); err != nil {
		t.Fatalf("share: %v", err)
	}
	if err := f.svc.Unshare(f.t.Context(), f.orgA, f.projA1, created.ID, f.projA2, f.actor); err != nil {
		t.Fatalf("unshare: %v", err)
	}
	if err := f.svc.Delete(f.t.Context(), f.orgA, f.projA1, created.ID, time.Time{}, f.actor); err != nil {
		t.Fatalf("delete: %v", err)
	}
	want := []string{ActionCreated, ActionUpdated, ActionUpdated, ActionShared, ActionUnshared, ActionDeleted}
	if len(f.store.entries) != len(want) {
		t.Fatalf("%d audit entries, want %d", len(f.store.entries), len(want))
	}
	for i, e := range f.store.entries {
		if e.Action != want[i] {
			t.Errorf("entry %d action = %s, want %s", i, e.Action, want[i])
		}
		if e.Actor.PrincipalID != f.actor.PrincipalID || e.Actor.Kind != f.actor.Kind {
			t.Errorf("entry %d actor = %+v, want %+v", i, e.Actor, f.actor)
		}
		if e.Chain.Org() != f.orgA.UUID() {
			t.Errorf("entry %d is in the chain of another organization", i)
		}
		for _, v := range []string{secret, "rotated-" + secret, "accessToken", "phoneNumberId"} {
			if bytes.Contains(e.Body, []byte(v)) {
				t.Errorf("entry %d (%s) carries %q", i, e.Action, v)
			}
		}
	}
	if !bytes.Contains(f.store.entries[1].Body, []byte(`"version":2`)) || !bytes.Contains(f.store.entries[1].Body, []byte(`"secret"`)) {
		t.Errorf("the replacement entry does not name the secret change and version 2: %s", f.store.entries[1].Body)
	}
	if !bytes.Contains(f.store.entries[2].Body, []byte(`"name"`)) {
		t.Errorf("the update entry does not name the changed field: %s", f.store.entries[2].Body)
	}
}

// TestOwnerProjectIsOrgScoped: the resolver that authorizes a request naming a credential answers with the owning
// project inside the organization, and with not found for another organization or a malformed identifier.
func TestOwnerProjectIsOrgScoped(t *testing.T) {
	f := newFixture(t)
	created := f.mustCreate(basicInput("v"))
	owner, err := f.svc.OwnerProject(f.t.Context(), f.orgA, created.ID.String())
	if err != nil || owner != f.projA1 {
		t.Fatalf("owner = %v, %v; want %v", owner, err, f.projA1)
	}
	if _, err := f.svc.OwnerProject(f.t.Context(), f.orgB, created.ID.String()); !errors.Is(err, identity.ErrNotFound) {
		t.Errorf("another organization = %v, want identity.ErrNotFound", err)
	}
	if _, err := f.svc.OwnerProject(f.t.Context(), f.orgA, "not-a-credential"); !errors.Is(err, identity.ErrNotFound) {
		t.Errorf("malformed identifier = %v, want identity.ErrNotFound", err)
	}
}
