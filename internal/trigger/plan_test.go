// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package trigger_test

import (
	"slices"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
	"github.com/wirvii/orcon/internal/trigger"
	"github.com/wirvii/orcon/internal/trigger/triggertest"
)

func TestProductionPattern(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		addr flow.TriggerAddress
		want string
	}{
		{"native", flow.TriggerAddress{Family: flow.AddressNative, Path: "orders"}, "/hooks/alpha/orders"},
		{"native with suffix", flow.TriggerAddress{Family: flow.AddressNative, Path: "orders", Suffix: "AbCdEfGhIjKlMnOpQrStUv"}, "/hooks/alpha/orders/AbCdEfGhIjKlMnOpQrStUv"},
		{"n8n", flow.TriggerAddress{Family: flow.AddressN8N, Path: "orders"}, "/webhook/orders"},
		{"n8n with parameters and webhook id", flow.TriggerAddress{Family: flow.AddressN8N, Path: "orders/:id", WebhookID: "w-1"}, "/webhook/w-1/orders/:id"},
		{"n8n webhook id without parameters is ignored", flow.TriggerAddress{Family: flow.AddressN8N, Path: "orders", WebhookID: "w-1"}, "/webhook/orders"},
		{"n8n parameters without webhook id", flow.TriggerAddress{Family: flow.AddressN8N, Path: "orders/:id"}, "/webhook/orders/:id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := trigger.ProductionPattern(tt.addr, "alpha"); got != tt.want {
				t.Errorf("ProductionPattern = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStaticPrefixAndOverlap(t *testing.T) {
	t.Parallel()
	if got := trigger.StaticPrefix("/hooks/alpha/orders/:id/lines"); got != "/hooks/alpha/orders" {
		t.Errorf("StaticPrefix with a parameter = %q", got)
	}
	if got := trigger.StaticPrefix("/hooks/alpha/orders"); got != "/hooks/alpha/orders" {
		t.Errorf("StaticPrefix without a parameter = %q", got)
	}
	overlaps := []struct {
		a, b string
		want bool
	}{
		{"/hooks/p/orders/new", "/hooks/p/orders/:id", true},
		{"/hooks/p/orders/:id", "/hooks/p/orders/:name", true},
		{"/hooks/p/orders/new", "/hooks/p/orders/new", false}, // equal patterns collide, they do not overlap
		{"/hooks/p/orders/new", "/hooks/p/orders/old", false},
		{"/hooks/p/orders", "/hooks/p/orders/:id", false},
	}
	for _, o := range overlaps {
		if got := trigger.Overlaps(o.a, o.b); got != o.want {
			t.Errorf("Overlaps(%q, %q) = %v, want %v", o.a, o.b, got, o.want)
		}
	}
}

func TestParseDuration(t *testing.T) {
	t.Parallel()
	good := map[string]time.Duration{
		"PT30S":   30 * time.Second,
		"PT1M":    time.Minute,
		"PT1H30M": 90 * time.Minute,
		"P2D":     48 * time.Hour,
		"P1W":     7 * 24 * time.Hour,
		"P1DT12H": 36 * time.Hour,
		"PT0.5S":  500 * time.Millisecond,
	}
	for in, want := range good {
		got, err := trigger.ParseDuration(in)
		if err != nil || got != want {
			t.Errorf("ParseDuration(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "P", "PT", "30S", "P1Y", "P1M", "PT-5S", "1D"} {
		if _, err := trigger.ParseDuration(bad); err == nil {
			t.Errorf("ParseDuration(%q) accepted a bad duration", bad)
		}
	}
}

func TestBuildSpecs(t *testing.T) {
	t.Parallel()
	raw := triggertest.Build(triggertest.DocSpec{Name: "specs", Starts: []triggertest.Node{
		{ID: "hook", Block: `{"family":"WEBHOOK","addresses":[{"family":"NATIVE","path":"a","methods":["POST","GET","POST"]},{"family":"N8N","path":"b","responseProfile":"NATIVE"}]}`},
		{ID: "plain"}, // no trigger block: not a registry entry
		{ID: "cron", Type: triggertest.TypeSchedule, Block: `{"family":"SCHEDULE","schedule":{"rules":[{"kind":"HOURS"}]}}`},
	}})
	value, err := jsonvalue.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := flow.Read(value)
	if err != nil {
		t.Fatal(err)
	}
	specs, err := trigger.BuildSpecs(&doc, "alpha")
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 2 || specs[0].NodeID != "hook" || specs[1].NodeID != "cron" {
		t.Fatalf("specs = %+v, want the two nodes with a trigger block in document order", specs)
	}
	if len(specs[0].ConfigurationHash) != 64 || specs[0].ConfigurationHash == specs[1].ConfigurationHash {
		t.Errorf("configuration hashes = %q and %q", specs[0].ConfigurationHash, specs[1].ConfigurationHash)
	}
	a, b := specs[0].Addresses[0], specs[0].Addresses[1]
	if !slices.Equal(a.Methods, []string{"GET", "POST"}) || a.ResponseProfile != flow.ProfileNative || a.Pattern != "/hooks/alpha/a" {
		t.Errorf("first address = %+v, want sorted unique methods and the native profile", a)
	}
	if b.Pattern != "/webhook/b" || b.ResponseProfile != flow.ProfileNative || !slices.Equal(b.Methods, []string{"POST"}) {
		t.Errorf("second address = %+v, want the n8n path with the declared profile and the default method", b)
	}
	if len(specs[1].Addresses) != 0 {
		t.Errorf("a schedule has addresses: %+v", specs[1].Addresses)
	}
}
