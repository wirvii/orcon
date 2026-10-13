// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSplitTokens(t *testing.T) {
	tests := map[string][]string{
		"GetAPIKeyByHash":     {"get", "api", "key", "by", "hash"},
		"GetFlowRef":          {"get", "flow", "ref"},
		"ListExecutions":      {"list", "executions"},
		"CountHTTPStatus":     {"count", "http", "status"},
		"GetSha256Digest":     {"get", "sha256", "digest"},
		"GetN8nNode":          {"get", "n8n", "node"},
		"Get":                 {"get"},
		"retry_cnt":           {"retry", "cnt"},
		"api_key":             {"api", "key"},
		"create_org":          {"create", "org"},
		"fts5":                {"fts5"},
		"ix_flow_org_id_name": {"ix", "flow", "org", "id", "name"},
	}
	for name, want := range tests {
		if got := splitTokens(name); !reflect.DeepEqual(got, want) {
			t.Errorf("splitTokens(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestSplitTokensOauth(t *testing.T) {
	if got, want := splitTokens("UpsertOauthTokenHash"), []string{"upsert", "oauth", "token", "hash"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func testPolicy(t *testing.T) *policy {
	t.Helper()
	p, fs := loadPolicy(realPolicy)
	if len(fs) != 0 {
		t.Fatalf("the repository policy has findings: %v", fs)
	}
	return p
}

func TestAbbreviationDecision(t *testing.T) {
	p := testPolicy(t)
	tests := []struct {
		name   string
		tokens []string
		prefix string
		denied []string
	}{
		{"allowed acronyms pass", splitTokens("api_key"), "", nil},
		{"allowed acronym with digits", splitTokens("sha256"), "", nil},
		{"digits only pass", []string{"2"}, "", nil},
		{"denied token is a finding", splitTokens("retry_cnt"), "", []string{"cnt"}},
		{"unlisted token is accepted", splitTokens("widget"), "", nil},
		{"prefix token in first position", splitTokens("pk_flow"), "pk", nil},
		{"prefix token in another position", splitTokens("flow_pk"), "pk", []string{"pk"}},
		{"prefix of another kind", splitTokens("ix_flow"), "pk", []string{"ix"}},
		{"prefix without a kind", splitTokens("pk_flow"), "", []string{"pk"}},
		{"two denied tokens", splitTokens("max_seq"), "", []string{"max", "seq"}},
		{"query name", splitTokens("GetFlowRef"), "", []string{"ref"}},
		{"query name with acronyms", splitTokens("GetAPIKeyByHash"), "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			for _, h := range p.abbreviationFindings(tt.tokens, tt.prefix) {
				got = append(got, h.token)
			}
			if !reflect.DeepEqual(got, tt.denied) {
				t.Fatalf("denied = %v, want %v", got, tt.denied)
			}
		})
	}
}

func TestAbbreviationHitCarriesExpansion(t *testing.T) {
	hits := testPolicy(t).abbreviationFindings([]string{"ref"}, "")
	if len(hits) != 1 || hits[0].expansion != "reference" {
		t.Fatalf("hits = %+v, want the expansion \"reference\"", hits)
	}
}

// policyDirWith copies the real policy and replaces the given files.
func policyDirWith(t *testing.T, overrides map[string]*string) string {
	t.Helper()
	dir := t.TempDir()
	copyDir(t, realPolicy, dir)
	for name, content := range overrides {
		if content == nil {
			if err := os.Remove(filepath.Join(dir, name)); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(*content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func policyFindingsFor(t *testing.T, overrides map[string]*string) []string {
	t.Helper()
	p, fs := loadPolicy(policyDirWith(t, overrides))
	fs = append(fs, checkNamingLists(p)...)
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.String())
	}
	return out
}

func TestRepositoryPolicyIsWellFormed(t *testing.T) {
	if got := policyFindingsFor(t, nil); len(got) != 0 {
		t.Fatalf("findings in the repository policy: %v", got)
	}
	p := testPolicy(t)
	if len(p.allowed) != 25 {
		t.Errorf("allowed_acronyms.txt holds %d tokens, NM-3 says 25", len(p.allowed))
	}
	if _, ok := p.softDelete["flow"]; !ok || len(p.softDelete) != 1 {
		t.Errorf("soft_delete_tables.txt = %v, want exactly flow", p.softDelete)
	}
	for _, tok := range []string{"ref", "max", "seq", "ms"} {
		if _, ok := p.denied[tok]; !ok {
			t.Errorf("abbreviation_denylist.txt lacks %q", tok)
		}
	}
}

func TestNamingListLoader(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]*string
		want  string // substring of one finding
	}{
		{"token in both lists", map[string]*string{fileAllowedAcronyms: str("api  # application programming interface\nref  # reference\n")}, `token "ref" is in both`},
		{"line without expansion", map[string]*string{fileAllowedAcronyms: str("api\n")}, "has no \"# <expansion>\""},
		{"line with an empty expansion", map[string]*string{fileAllowedAcronyms: str("api  #\n")}, "has no \"# <expansion>\""},
		{"token with uppercase", map[string]*string{fileAllowedAcronyms: str("API  # application programming interface\n")}, "does not match ^[a-z][a-z0-9]*$"},
		{"token with an underscore", map[string]*string{fileAllowedAcronyms: str("api_key  # key\n")}, "does not match ^[a-z][a-z0-9]*$"},
		{"token starting with a digit", map[string]*string{fileAllowedAcronyms: str("8n  # eight\n")}, "does not match ^[a-z][a-z0-9]*$"},
		{"duplicate allowed token", map[string]*string{fileAllowedAcronyms: str("api  # one\napi  # two\n")}, `duplicate token "api"`},
		{"allowed list out of order", map[string]*string{fileAllowedAcronyms: str("url  # locator\napi  # interface\n")}, `"api" is out of alphabetical order`},
		{"denied list out of order", map[string]*string{fileDenylist: str("ref  # reference\nack  # acknowledgement\n")}, `"ack" is out of alphabetical order`},
		{"duplicate denied token", map[string]*string{fileDenylist: str("ack  # one\nack  # two\n")}, `duplicate token "ack"`},
		{"missing allowed list", map[string]*string{fileAllowedAcronyms: nil}, "policy file is missing"},
		{"missing denied list", map[string]*string{fileDenylist: nil}, "policy file is missing"},
		{"missing hot patterns", map[string]*string{fileHotQueryPatterns: nil}, "policy file is missing"},
		{"invalid hot pattern", map[string]*string{fileHotQueryPatterns: str("^(Claim\n")}, "not a valid regular expression"},
		{"soft delete line without reason", map[string]*string{fileSoftDelete: str("flow\n")}, "malformed line"},
		{"query verb not capitalised", map[string]*string{fileQueryVerbs: str("get  # read\n")}, "does not match"},
		{"dialect only entry without dialect", map[string]*string{fileDialectOnly: str("GetThing  # why\n")}, "does not match"},
		{"external id without table", map[string]*string{fileExternalIDs: str("node_id  # inside a document\n")}, "does not match"},
		{"reserved word with uppercase", map[string]*string{fileReservedWords: str("User\n")}, "one lowercase token per line"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := policyFindingsFor(t, tt.files)
			if !contains(got, tt.want) {
				t.Fatalf("findings %v lack %q", got, tt.want)
			}
		})
	}
}

func TestCommentsAndBlankLinesAreIgnored(t *testing.T) {
	got := policyFindingsFor(t, map[string]*string{fileAllowedAcronyms: str("# comment\n\napi  # application programming interface\n   \n# more\nurl  # locator\n")})
	if len(got) != 0 {
		t.Fatalf("findings: %v", got)
	}
}

func TestLoadExceptions(t *testing.T) {
	now := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		content string
		wantHas bool
		finding string
	}{
		{"valid", "JS-5  postgres:GetOrg  needs a containment lookup  2027-01-31  approved-by:owner\n", true, ""},
		{"valid through its date", "JS-5  postgres:GetOrg  reason  2026-10-13  approved-by:owner\n", true, ""},
		{"expired", "JS-5  postgres:GetOrg  reason  2026-10-12  approved-by:owner\n", false, "expired on 2026-10-12"},
		{"missing approved-by", "JS-5  postgres:GetOrg  reason  2027-01-31\n", false, "malformed exception"},
		{"empty approver", "JS-5  postgres:GetOrg  reason  2027-01-31  approved-by:\n", false, "malformed exception"},
		{"bad date", "JS-5  postgres:GetOrg  reason  2027-13-45  approved-by:owner\n", false, "not a valid date"},
		{"bad dialect", "JS-5  mysql:GetOrg  reason  2027-01-31  approved-by:owner\n", false, "malformed exception"},
		{"single spaces", "JS-5 postgres:GetOrg reason 2027-01-31 approved-by:owner\n", false, "malformed exception"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := policyDirWith(t, map[string]*string{fileExceptions: str(tt.content)})
			ex, fs := loadExceptions(dir, now)
			if got := ex.has("JS-5", "postgres", "GetOrg"); got != tt.wantHas {
				t.Errorf("has = %v, want %v", got, tt.wantHas)
			}
			var lines []string
			for _, f := range fs {
				lines = append(lines, f.String())
			}
			if tt.finding == "" && len(lines) != 0 {
				t.Errorf("unexpected findings: %v", lines)
			}
			if tt.finding != "" && !contains(lines, tt.finding) {
				t.Errorf("findings %v lack %q", lines, tt.finding)
			}
		})
	}
	if ex, _ := loadExceptions(policyDirWith(t, nil), now); ex.has("JS-5", "postgres", "GetOrg") || len(ex.list) != 0 {
		t.Error("the repository exceptions file must be empty at birth")
	}
	if (*exceptions)(nil).has("JS-5", "postgres", "x") {
		t.Error("a nil exceptions set grants nothing")
	}
}

func TestFindingString(t *testing.T) {
	f := Finding{File: "a/b.sql", Line: 7, Rule: "NM-3", Msg: "message", Fix: "do this"}
	if got, want := f.String(), "a/b.sql:7: NM-3 message (fix: do this)"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got := (Finding{File: "x", Line: 1, Rule: "R", Msg: "m"}).String(); strings.Contains(got, "fix") {
		t.Fatalf("no fix expected: %q", got)
	}
}
