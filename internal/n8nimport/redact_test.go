// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package n8nimport

import (
	"strings"
	"testing"
)

// plantedSecrets are the values the golden workflow carries as secrets. None
// of them may appear in the flow document, the report or the findings.
var plantedSecrets = []string{
	"PLANTED-bearer-7f3a9c21d8e4",
	"PLANTEDxapikeyQ8w2Z9rT4vB6nM1kJ",
	"cGxhbnRlZDpzZWNyZXQ=",
	"PLANTEDapikeyInBody",
	"q8Z3xT0pL9mV2sK7wR4nB6hY1cD5fG0j",
	"PLANTEDurltoken123",
	"PLANTEDaccessTokenValue",
	"PLANTEDclientSecretValue",
	"PLANTEDpassword-hunter2",
	"sk-PLANTEDsticky00112233aabb",
	"sk-PLANTEDcondition99887766",
	"sk-PLANTEDvendor0123456789abcdef",
}

// secretFindings collects the secret findings of one node.
func secretFindings(conv *Conversion, node string) []Finding {
	var out []Finding
	for _, n := range conv.Nodes {
		if n.Name != node {
			continue
		}
		for _, f := range n.Findings {
			if f.Code == FindSecretValue {
				out = append(out, f)
			}
		}
	}
	return out
}

func fieldsOf(fs []Finding) []string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = f.Field
	}
	return out
}

func TestPlantedSecretsNeverSurvive(t *testing.T) {
	conv := convertFixture(t, "planted-secrets")
	flowJSON, err := conv.FlowJSON()
	if err != nil {
		t.Fatal(err)
	}
	var everything strings.Builder
	everything.Write(flowJSON)
	everything.WriteString(conv.Markdown())
	for _, f := range conv.Findings {
		everything.WriteString(f.Message + " " + f.Field)
	}
	for _, n := range conv.Nodes {
		for _, f := range n.Findings {
			everything.WriteString(f.Message + " " + f.Field)
		}
	}
	for _, p := range conv.Problems {
		everything.WriteString(p)
	}
	out := everything.String()
	for _, secret := range plantedSecrets {
		if strings.Contains(out, secret) {
			t.Errorf("a planted secret survived the conversion: %q", secret)
		}
	}
	if strings.Contains(out, "PLANTED") {
		t.Errorf("a fragment of a planted secret survived the conversion")
	}
	if !strings.Contains(string(flowJSON), SecretMarker) {
		t.Errorf("the flow has no %s marker", SecretMarker)
	}
}

func TestPlantedSecretsAreReportedByField(t *testing.T) {
	conv := convertFixture(t, "planted-secrets")

	crm := fieldsOf(secretFindings(conv, "CRM request"))
	wantCRM := []string{
		"parameters.url",
		"parameters.headerParameters.parameters[0].value",
		"parameters.headerParameters.parameters[1].value",
		"parameters.headerParameters.parameters[2].value",
		"parameters.bodyParameters.parameters[0].value",
		"parameters.bodyParameters.parameters[1].value",
	}
	if strings.Join(crm, "|") != strings.Join(wantCRM, "|") {
		t.Errorf("CRM request secret fields = %v, want %v", crm, wantCRM)
	}

	set := fieldsOf(secretFindings(conv, "Save settings"))
	wantSet := []string{
		"parameters.assignments.assignments[0].value",
		"parameters.assignments.assignments[1].value",
		"parameters.assignments.assignments[2].value",
	}
	if strings.Join(set, "|") != strings.Join(wantSet, "|") {
		t.Errorf("Set secret fields = %v, want %v", set, wantSet)
	}

	if got := fieldsOf(secretFindings(conv, "Sync code")); len(got) != 1 || got[0] != "parameters.jsCode" {
		t.Errorf("Code secret fields = %v, want [parameters.jsCode]", got)
	}

	// The condition is refused, and the original node keeps the value only as
	// a marker: two findings, one per place.
	guard := secretFindings(conv, "Guard key")
	if len(guard) != 2 || !strings.Contains(guard[0].Message, "comparison value") {
		t.Errorf("the IF with a secret comparison value has findings %v", guard)
	}
	if outcomeOf(t, conv, "Guard key").Outcome != OutcomeUnsupported {
		t.Errorf("the IF with a secret comparison value must not convert")
	}

	if len(secretFindings(conv, "Start sync")) != 0 {
		t.Errorf("a node without secrets has secret findings")
	}
	var stickyReported bool
	for _, f := range conv.Findings {
		if f.Code == FindSecretValue && f.Node == "Ops note" {
			stickyReported = true
		}
	}
	if !stickyReported {
		t.Errorf("the sticky note with a key has no finding")
	}
}

func TestRedactionKeepsWhatIsNotASecret(t *testing.T) {
	conv := convertFixture(t, "planted-secrets")
	flowJSON, err := conv.FlowJSON()
	if err != nil {
		t.Fatal(err)
	}
	// Controls: plain header value, URL host, ordinary query parameter, a
	// numeric token count, a plain assignment and an expression reference.
	for _, kept := range []string{
		`application/json`,
		`crm.example.test`,
		`lang=es`,
		`"maxTokens":500`,
		`"value":"bogota"`,
		`={{ $json.lead }}`,
	} {
		if !strings.Contains(string(flowJSON), kept) {
			t.Errorf("non-secret %q was removed from the flow", kept)
		}
	}
}

func TestRedactText(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		changed bool
	}{
		{in: "plain words only", want: "plain words only"},
		{in: "Bearer abcdef123456", want: SecretMarker, changed: true},
		{in: "key sk-live0123456789abcdef here", want: "key " + SecretMarker + " here", changed: true},
		{in: "https://x.test/a?api_key=zzz9&lang=es", want: "https://x.test/a?api_key=" + SecretMarker + "&lang=es", changed: true},
		{in: "AKIA0123456789ABCDEF", want: SecretMarker, changed: true},
		{in: "https://example.test/items/0123456789012345678901234", want: "https://example.test/items/0123456789012345678901234"},
		{in: "550e8400-e29b-41d4-a716-446655440000", want: "550e8400-e29b-41d4-a716-446655440000"},
	}
	for _, tt := range tests {
		got, changed := redactText(tt.in)
		if got != tt.want || changed != tt.changed {
			t.Errorf("redactText(%q) = (%q, %v), want (%q, %v)", tt.in, got, changed, tt.want, tt.changed)
		}
	}
}

func TestSecretKeyNames(t *testing.T) {
	for _, name := range []string{"Authorization", "Proxy-Authorization", "X-API-Key", "api_key", "accessToken", "clientSecret", "password", "bearer"} {
		if !secretKey(name) {
			t.Errorf("secretKey(%q) = false", name)
		}
	}
	// maxTokens matches by name; it is harmless because only string values are
	// replaced, and the numeric count of the golden workflow is kept.
	for _, name := range []string{"url", "method", "Accept", "lead"} {
		if secretKey(name) {
			t.Errorf("secretKey(%q) = true", name)
		}
	}
}
