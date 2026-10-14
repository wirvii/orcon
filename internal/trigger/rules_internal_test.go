// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package trigger

import (
	"strings"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/id"
)

func TestValidateCron(t *testing.T) {
	t.Parallel()
	good := []string{
		"* * * * *", "0 9 * * 1-5", "*/15 * * * *", "0 0 1 JAN *", "0 0 * * SUN", "30 8 * * MON,WED,FRI",
		"0 0 9 * * *", "10-20/2 * * * *", "0 0 * * 7",
	}
	for _, expr := range good {
		if err := validateCron(expr); err != nil {
			t.Errorf("validateCron(%q) = %v, want it valid", expr, err)
		}
	}
	bad := map[string]string{
		"":              "fields",
		"* * *":         "fields",
		"* * * * * * *": "fields",
		"60 * * * *":    "minutes",
		"* 24 * * *":    "hours",
		"* * 0 * *":     "day of month",
		"* * * 13 *":    "month",
		"* * * * 8":     "day of week",
		"*/0 * * * *":   "step",
		"*/x * * * *":   "step",
		"5-1 * * * *":   "backwards",
		"a * * * *":     "minutes",
		"* * * FOO *":   "month",
	}
	for expr, field := range bad {
		err := validateCron(expr)
		if err == nil {
			t.Errorf("validateCron(%q) accepted a bad expression", expr)
			continue
		}
		if msg := err.Error(); !strings.Contains(msg, field) {
			t.Errorf("validateCron(%q) = %q, want it to mention %q", expr, msg, field)
		}
	}
}

func TestConnectorReferences(t *testing.T) {
	t.Parallel()
	s := &Service{ids: id.NewV7Generator(time.Now)}
	doc := &flow.Document{Nodes: []flow.Node{
		{ID: "a", Type: "acme.crm.create@1", Connector: &flow.ConnectorPin{Version: "1.2.0"}},
		{ID: "b", Type: "acme.crm.update@1", Connector: &flow.ConnectorPin{Version: "1.2.0"}},
		{ID: "c", Type: "orcon.http@1"},
	}}
	refs := s.connectorReferences(doc)
	if len(refs) != 1 || refs[0].ConnectorID != "acme.crm" || refs[0].ConnectorVersion != "1.2.0" || refs[0].ID.IsZero() {
		t.Fatalf("connectorReferences() = %+v, want one acme.crm 1.2.0 pin with an identity", refs)
	}
	if got := s.connectorReferences(&flow.Document{}); got != nil {
		t.Fatalf("a document without pins gave %+v", got)
	}
}
