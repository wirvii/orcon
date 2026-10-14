// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package examplestest_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/adapter/httpapi/examplestest"
	"github.com/wirvii/orcon/internal/adapter/httpapi/examplestest/scenarios"
)

const bundlePath = "../../../../packages/openapi/v1/openapi.bundle.json"

// contractScenarios returns the scenario names the examples of the contract use.
func contractScenarios(t *testing.T) []string {
	t.Helper()
	var doc struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(readFile(t, bundlePath), &doc); err != nil {
		t.Fatalf("parse the bundled contract: %v", err)
	}
	seen := map[string]bool{}
	for _, item := range doc.Paths {
		for _, raw := range item {
			var op struct {
				Examples []struct {
					Scenario string `json:"scenario"`
				} `json:"x-orcon-examples"`
			}
			if json.Unmarshal(raw, &op) != nil {
				continue // path-level members such as parameters are not operations
			}
			for _, e := range op.Examples {
				seen[e.Scenario] = true
			}
		}
	}
	var out []string
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// TestEveryExampleScenarioRegistered: every scenario an example names exists in
// the scenarios package, and the package holds no scenario no example uses.
func TestEveryExampleScenarioRegistered(t *testing.T) {
	registered := scenarios.All()
	used := contractScenarios(t)
	if len(used) == 0 {
		t.Fatal("the contract has no example scenario")
	}
	kebab := regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)
	for _, name := range used {
		if _, ok := registered[name]; !ok {
			t.Errorf("an example names the scenario %q and no such scenario is registered", name)
		}
		if !kebab.MatchString(name) {
			t.Errorf("scenario %q is not kebab-case", name)
		}
	}
	usedSet := map[string]bool{}
	for _, name := range used {
		usedSet[name] = true
	}
	for name := range registered {
		if !usedSet[name] {
			t.Errorf("the scenario %q is registered and no example uses it", name)
		}
	}
}

// fakeAPI answers the calls of the scenarios with canned bodies and checks each
// request against the contract, so a scenario cannot ask for something the
// contract does not describe.
type fakeAPI struct {
	t       *testing.T
	checker *examplestest.RequestChecker
	n       int
	calls   []string
}

var routeAnswers = []struct {
	method  string
	pattern *regexp.Regexp
	status  int
	body    string
}{
	{http.MethodPost, regexp.MustCompile(`^/api/v1/orgs/[^/]+/projects$`), 201, `{"id":"%s"}`},
	{http.MethodPost, regexp.MustCompile(`^/api/v1/projects/[^/]+/flows$`), 201, `{"id":"%s"}`},
	{http.MethodPut, regexp.MustCompile(`^/api/v1/flows/[^/]+/draft$`), 200, `{}`},
	{http.MethodPost, regexp.MustCompile(`^/api/v1/flows/[^/]+/publish$`), 200, `{}`},
	{http.MethodPost, regexp.MustCompile(`^/api/v1/flows/[^/]+/activate$`), 200, `{}`},
	{http.MethodPost, regexp.MustCompile(`^/api/v1/flows/[^/]+/executions$`), 200, `{"id":"%s"}`},
	{http.MethodGet, regexp.MustCompile(`^/api/v1/executions/[^/]+/nodes$`), 200, `{"items":[{"id":"%s"}]}`},
	{http.MethodPost, regexp.MustCompile(`^/api/v1/flows/[^/]+/test-sessions$`), 201, `{"id":"%s"}`},
}

func (f *fakeAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.calls = append(f.calls, r.Method+" "+r.URL.Path)
	if err := f.checker.Check(r.Method, r.URL.Path, r.URL.Query(), r.Header, body); err != nil {
		f.t.Errorf("%s %s: %v", r.Method, r.URL.Path, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	for _, a := range routeAnswers {
		if a.method == r.Method && a.pattern.MatchString(r.URL.Path) {
			f.n++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(a.status)
			_, _ = fmt.Fprintf(w, a.body, fmt.Sprintf("0191f6a2-3c4d-7e8f-9a0b-%012x", f.n))
			return
		}
	}
	f.t.Errorf("%s %s: the fake has no answer for it", r.Method, r.URL.Path)
	http.NotFound(w, r)
}

// TestScenariosFollowTheContract runs every scenario against a fake that
// checks each request with the contract: a scenario that sends a field the
// contract does not know, a missing If-Match or a body that does not validate
// fails here, before any handler exists.
func TestScenariosFollowTheContract(t *testing.T) {
	const org = "0191f6a2-3c4d-7e8f-9a0b-aaaaaaaaaaaa"
	checker, err := examplestest.NewRequestChecker(readFile(t, bundlePath), "/api/v1")
	if err != nil {
		t.Fatalf("NewRequestChecker: %v", err)
	}
	for name, run := range scenarios.All() {
		t.Run(name, func(t *testing.T) {
			fake := &fakeAPI{t: t, checker: checker}
			srv := httptest.NewServer(fake)
			defer srv.Close()
			transport := &http.Transport{}
			defer transport.CloseIdleConnections()
			api := examplestest.NewClient(srv.URL, "/api/v1", "", &http.Client{Transport: transport}).WithOrg(org)
			bindings, err := run(t.Context(), api)
			if errors.Is(err, scenarios.ErrNotSeedable) {
				return // the scenario says why it cannot run yet
			}
			if err != nil {
				t.Fatalf("scenario: %v (calls %v)", err, fake.calls)
			}
			if bindings["0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a71"] != org {
				t.Errorf("the organization is not bound to the credential's organization: %v", bindings)
			}
		})
	}
}

func TestNotSeedableScenariosSayWhy(t *testing.T) {
	api := examplestest.NewClient("http://127.0.0.1:1", "/api/v1", "", nil).WithOrg("org")
	for _, name := range []string{"execution-with-human-task", "execution-with-uncertain-operation"} {
		_, err := scenarios.All()[name](t.Context(), api)
		if !errors.Is(err, scenarios.ErrNotSeedable) {
			t.Errorf("%s: err = %v, want ErrNotSeedable", name, err)
		}
	}
}

func TestRequestCheckerRejectsWhatTheContractForbids(t *testing.T) {
	checker, err := examplestest.NewRequestChecker(readFile(t, bundlePath), "/api/v1")
	if err != nil {
		t.Fatalf("NewRequestChecker: %v", err)
	}
	json := http.Header{"Content-Type": {"application/json"}}
	tests := []struct {
		name   string
		method string
		path   string
		header http.Header
		body   string
	}{
		{"unknown route", http.MethodGet, "/api/v1/nowhere", nil, ""},
		{"wrong method", http.MethodDelete, "/api/v1/flows/x/publish", nil, ""},
		{"missing If-Match", http.MethodPut, "/api/v1/flows/x/draft", json, `{"document":{"format":"orcon/flow@1","nodes":[],"edges":[]}}`},
		{"unknown body member", http.MethodPost, "/api/v1/projects/x/flows", json, `{"name":"a","surprise":1}`},
		{"missing body", http.MethodPost, "/api/v1/projects/x/flows", json, ``},
		{"wrong media type", http.MethodPost, "/api/v1/projects/x/flows", http.Header{"Content-Type": {"text/plain"}}, `{"name":"a"}`},
		{"body for a bodyless operation", http.MethodGet, "/api/v1/flows/x", json, `{}`},
		{"body that is not JSON", http.MethodPost, "/api/v1/projects/x/flows", json, `{`},
	}
	for _, tt := range tests {
		err := checker.Check(tt.method, tt.path, nil, tt.header, bytes.NewBufferString(tt.body).Bytes())
		if !errors.Is(err, examplestest.ErrRequestMismatch) {
			t.Errorf("%s: err = %v, want ErrRequestMismatch", tt.name, err)
		}
	}
	good := http.Header{"Content-Type": {"application/json"}, "If-Match": {`"1"`}}
	if err := checker.Check(http.MethodPut, "/api/v1/flows/x/draft", nil, good, []byte(`{"document":{"format":"orcon/flow@1","nodes":[],"edges":[]}}`)); err != nil {
		t.Errorf("a valid request was rejected: %v", err)
	}
	if err := checker.Check(http.MethodGet, "/api/v1/projects/x/flows", map[string][]string{"nope": {"1"}}, nil, nil); !errors.Is(err, examplestest.ErrRequestMismatch) || !strings.Contains(err.Error(), "nope") {
		t.Errorf("an unknown query parameter was accepted: %v", err)
	}
}
