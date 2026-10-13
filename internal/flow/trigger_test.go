// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow_test

import (
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// startNode returns the JSON text of a startEvent node with the given trigger block.
func startNode(block string) string {
	return `{"id":"s","name":"S","element":"startEvent","type":"orcon.webhook@1","parameters":{"x":1},"trigger":` + block + `}`
}

func readTrigger(t *testing.T, block string) (flow.Document, error) {
	t.Helper()
	v, err := jsonvalue.Parse([]byte(docWith("", startNode(block), "", "")))
	if err != nil {
		t.Fatal(err)
	}
	return flow.FromValue(v)
}

const fullWebhook = `{"family":"WEBHOOK","addresses":[{"family":"N8N","path":"orders/:id","webhookId":"w1","methods":["POST","PUT"],"responseProfile":"N8N"},{"family":"NATIVE","path":"orders","suffix":"AbCdEfGhIjKlMnOpQrStUv","responseProfile":"NATIVE"}],"response":{"mode":"LAST_NODE","waitTimeout":"PT30S"},"dedupe":{"source":"HEADER","name":"X-Id","window":"PT24H"}}`

func TestTriggerBlockRoundTrips(t *testing.T) {
	t.Parallel()
	blocks := map[string]string{
		"webhook":  fullWebhook,
		"schedule": `{"family":"SCHEDULE","schedule":{"timeZone":"Europe/Madrid","rules":[{"kind":"CRON","cron":"0 9 * * 1-5"},{"kind":"WEEKS","every":2,"at":"08:30","weekdays":["MONDAY","FRIDAY"]},{"kind":"MONTHS","dayOfMonth":15,"at":"07:00"}],"missed":{"policy":"CATCH_UP_ONCE","marginHours":6}}}`,
		"poll":     `{"family":"POLL","poll":{"rules":[{"kind":"MINUTES","every":5}]}}`,
		"listen":   `{"family":"LISTEN","listen":{"maxInFlight":10,"hostTag":"edge-1","localPath":"/data/in"},"dedupe":{"source":"CONNECTOR"}}`,
		"app":      `{"family":"APP_EVENT","registration":{}}`,
		"internal": `{"family":"INTERNAL","inputSchema":{"type":"object"},"callers":["01930000-0000-7000-8000-0000000000bb"]}`,
		"project":  `{"family":"INTERNAL","callers":"PROJECT"}`,
	}
	for name, block := range blocks {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			doc, err := readTrigger(t, block)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			want, err := jsonvalue.Parse([]byte(block))
			if err != nil {
				t.Fatal(err)
			}
			back, ok := doc.Value().Get("nodes")
			if !ok {
				t.Fatal("no nodes in the written document")
			}
			written, ok := back.Items()[0].Get("trigger")
			if !ok {
				t.Fatal("the trigger block was not written back")
			}
			if !jsonvalue.Equal(written, want) {
				w, _ := jsonvalue.Marshal(written)
				t.Errorf("written block differs:\n got %s\nwant %s", w, block)
			}
		})
	}
}

func TestTriggerBlockShapeErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		block string
		want  string
	}{
		{"unknown member", `{"family":"WEBHOOK","surprise":1}`, `unknown property "surprise"`},
		{"missing family", `{"addresses":[]}`, `missing required property "family"`},
		{"callers text", `{"family":"INTERNAL","callers":"ALL"}`, "PROJECT"},
		{"callers empty list", `{"family":"INTERNAL","callers":[]}`, "must not be empty"},
		{"rule without kind", `{"family":"POLL","poll":{"rules":[{"every":2}]}}`, `missing required property "kind"`},
		{"every zero", `{"family":"POLL","poll":{"rules":[{"kind":"MINUTES","every":0}]}}`, "must be an integer from"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := readTrigger(t, tt.block)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("read error = %v, want it to contain %q", err, tt.want)
			}
		})
	}
}

func TestTriggerBlockValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		block string
		code  string
	}{
		{"unknown family", `{"family":"webhook","addresses":[{"family":"NATIVE","path":"a"}]}`, flow.CodeEnum},
		{"webhook without address", `{"family":"WEBHOOK"}`, flow.CodeTrigger},
		{"schedule block on a webhook", `{"family":"WEBHOOK","addresses":[{"family":"NATIVE","path":"a"}],"schedule":{"rules":[{"kind":"MINUTES"}]}}`, flow.CodeTrigger},
		{"leading slash", `{"family":"WEBHOOK","addresses":[{"family":"NATIVE","path":"/a"}]}`, flow.CodeValue},
		{"path escapes", `{"family":"WEBHOOK","addresses":[{"family":"NATIVE","path":"a/../b"}]}`, flow.CodeValue},
		{"lowercase address family", `{"family":"WEBHOOK","addresses":[{"family":"native","path":"a"}]}`, flow.CodeEnum},
		{"bad method", `{"family":"WEBHOOK","addresses":[{"family":"NATIVE","path":"a","methods":["post"]}]}`, flow.CodeEnum},
		{"duplicate method", `{"family":"WEBHOOK","addresses":[{"family":"NATIVE","path":"a","methods":["POST","POST"]}]}`, flow.CodeValue},
		{"suffix on n8n address", `{"family":"WEBHOOK","addresses":[{"family":"N8N","path":"a","suffix":"AbCdEfGhIjKlMnOpQrStUv"}]}`, flow.CodeValue},
		{"short suffix", `{"family":"WEBHOOK","addresses":[{"family":"NATIVE","path":"a","suffix":"abc"}]}`, flow.CodeValue},
		{"bad mode", `{"family":"WEBHOOK","addresses":[{"family":"NATIVE","path":"a"}],"response":{"mode":"later"}}`, flow.CodeEnum},
		{"bad timeout", `{"family":"WEBHOOK","addresses":[{"family":"NATIVE","path":"a"}],"response":{"mode":"LAST_NODE","waitTimeout":"30s"}}`, flow.CodeValue},
		{"header dedupe without name", `{"family":"WEBHOOK","addresses":[{"family":"NATIVE","path":"a"}],"dedupe":{"source":"HEADER"}}`, flow.CodeValue},
		{"connector dedupe with name", `{"family":"LISTEN","listen":{},"dedupe":{"source":"CONNECTOR","name":"x"}}`, flow.CodeValue},
		{"dedupe on a schedule", `{"family":"SCHEDULE","schedule":{"rules":[{"kind":"MINUTES"}]},"dedupe":{"source":"CONNECTOR"}}`, flow.CodeTrigger},
		{"catch up without margin", `{"family":"SCHEDULE","schedule":{"rules":[{"kind":"MINUTES"}],"missed":{"policy":"CATCH_UP_ONCE"}}}`, flow.CodeValue},
		{"margin with skip", `{"family":"SCHEDULE","schedule":{"rules":[{"kind":"MINUTES"}],"missed":{"policy":"SKIP","marginHours":3}}}`, flow.CodeValue},
		{"bad time of day", `{"family":"SCHEDULE","schedule":{"rules":[{"kind":"DAYS","at":"25:00"}]}}`, flow.CodeValue},
		{"bad weekday", `{"family":"SCHEDULE","schedule":{"rules":[{"kind":"WEEKS","weekdays":["Mon"]}]}}`, flow.CodeEnum},
		{"cron with three fields", `{"family":"SCHEDULE","schedule":{"rules":[{"kind":"CRON","cron":"* * *"}]}}`, flow.CodeValue},
		{"cron characters", `{"family":"SCHEDULE","schedule":{"rules":[{"kind":"CRON","cron":"* * * * $"}]}}`, flow.CodeValue},
		{"cron on a minutes rule", `{"family":"SCHEDULE","schedule":{"rules":[{"kind":"MINUTES","cron":"* * * * *"}]}}`, flow.CodeValue},
		{"empty rules", `{"family":"POLL","poll":{"rules":[]}}`, flow.CodeValue},
		{"bad host tag", `{"family":"LISTEN","listen":{"hostTag":"-bad"}}`, flow.CodeValue},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc, err := readTrigger(t, tt.block)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			report := flow.Validate(&doc)
			if !has(report, tt.code, flow.SeverityError) {
				t.Errorf("Validate did not report %s as an error; issues: %v", tt.code, report.Issues)
			}
		})
	}
}

func TestTriggerBlockOnlyOnStartEvents(t *testing.T) {
	t.Parallel()
	node := strings.Replace(startNode(`{"family":"INTERNAL"}`), `"startEvent"`, `"task"`, 1)
	v, err := jsonvalue.Parse([]byte(docWith("", node, "", "")))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := flow.FromValue(v)
	if err != nil {
		t.Fatal(err)
	}
	if report := flow.Validate(&doc); !has(report, flow.CodeBlock, flow.SeverityError) {
		t.Errorf("a trigger block on a task was not rejected: %v", report.Issues)
	}
}

func TestTriggerConfigurationHash(t *testing.T) {
	t.Parallel()
	hash := func(block, params string) string {
		t.Helper()
		doc, err := readTrigger(t, block)
		if err != nil {
			t.Fatal(err)
		}
		n := &doc.Nodes[0]
		if params != "" {
			n.Parameters = obj(jsonvalue.Member{Key: "y", Value: jsonvalue.String(params)})
		}
		h, err := n.TriggerConfigurationHash()
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	base := hash(fullWebhook, "")
	if len(base) != 64 {
		t.Fatalf("hash = %q, want 64 hexadecimal digits", base)
	}
	if again := hash(fullWebhook, ""); again != base {
		t.Error("the hash of the same configuration changed")
	}
	// Member order in the source text does not matter: the form is canonical.
	reordered := `{"dedupe":{"window":"PT24H","name":"X-Id","source":"HEADER"},` + strings.TrimPrefix(fullWebhook, `{`)
	reordered = strings.Replace(reordered, `,"dedupe":{"source":"HEADER","name":"X-Id","window":"PT24H"}`, "", 1)
	if got := hash(reordered, ""); got != base {
		t.Error("the hash depends on member order")
	}
	if got := hash(strings.Replace(fullWebhook, "PT30S", "PT45S", 1), ""); got == base {
		t.Error("changing the response timeout did not change the hash")
	}
	if got := hash(fullWebhook, "other"); got == base {
		t.Error("changing the node parameters did not change the hash")
	}
	doc, err := readTrigger(t, `{"family":"INTERNAL"}`)
	if err != nil {
		t.Fatal(err)
	}
	doc.Nodes[0].Trigger = nil
	if h, err := doc.Nodes[0].TriggerConfigurationHash(); err != nil || h != "" {
		t.Errorf("hash of a node without a trigger = %q, %v; want empty", h, err)
	}
}
