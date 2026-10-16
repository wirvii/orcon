// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/wirvii/orcon/internal/adapter/httpapi/apigen"
	"github.com/wirvii/orcon/internal/connector"
	"github.com/wirvii/orcon/internal/connector/authoring"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// catalogShopManifest is the manifest of a made-up shop connector with an action node that has one
// operation of every safety level and a trigger node.
const catalogShopManifest = `{
  "format": "orcon/connector@1",
  "id": "acme.shop",
  "version": "VERSION",
  "kind": "DECLARATIVE",
  "displayName": "Acme Shop",
  "description": "Orders of the Acme Shop service.",
  "publisher": { "name": "Acme", "url": "https://acme.example.com", "contact": "support@acme.example.com" },
  "license": "Apache-2.0",
  "source": { "repository": "https://github.com/acme/orcon-shop", "commit": "0123456789abcdef0123456789abcdef01234567" },
  "compat": { "engineApi": ">=1.0.0 <2.0.0" },
  "nodes": [
    {
      "name": "order",
      "type": "acme.shop.order@1",
      "displayName": "Order",
      "group": "ACTION",
      "operations": [
        { "name": "order.get", "displayName": "Get an order", "purity": "READ", "itemMode": "PER_ITEM", "rateLimit": "api" },
        {
          "name": "order.create", "displayName": "Create an order", "purity": "EFFECT", "itemMode": "PER_ITEM",
          "safety": {
            "effect": "WRITE", "level": "IDEMPOTENCY_KEY",
            "idempotency": { "mode": "PROVIDER_KEY", "location": { "in": "HEADER", "name": "Idempotency-Key" }, "ttl": "PT24H", "scope": "CREDENTIAL" },
            "onUncertain": "RESEND_WITH_KEY", "retrySafe": true, "proofLevel": "TESTED"
          }
        },
        {
          "name": "order.send", "displayName": "Send an order", "purity": "EFFECT", "itemMode": "BATCH",
          "safety": {
            "effect": "WRITE", "level": "UNSAFE", "onUncertain": "SUSPEND", "retrySafe": false,
            "batch": { "maxItems": 50, "perItemResults": true },
            "manualCheck": { "description": "Open the order {{ $parameter.orderId }} in the shop and check it was sent.", "url": "https://shop.example.com/orders/{{ $parameter.orderId }}" },
            "proofLevel": "DECLARED"
          }
        }
      ],
      "parameters": { "orderId": { "type": "STRING", "required": true } },
      "outputs": ["main"],
      "n8n": { "types": [{ "name": "n8n-nodes-base.acmeShop", "typeVersions": [1, 1.1] }] }
    },
    {
      "name": "orderCreated",
      "type": "acme.shop.orderCreated@1",
      "displayName": "Order created",
      "group": "TRIGGER",
      "trigger": { "family": "POLL", "delivery": "CURSOR_EXACT", "dedupeKeySource": "JSON_PATH", "maxPayload": 1048576 }
    }
  ],
  "credentials": [
    {
      "id": "apiKey", "displayName": "Acme API key",
      "fields": [{ "name": "key", "type": "STRING", "sensitive": true, "required": true }],
      "scheme": { "type": "API_KEY", "in": "HEADER", "name": "X-Api-Key" },
      "test": "order.get"
    }
  ],
  "permissions": {
    "network": { "domains": ["api.shop.example.com"], "schemes": ["https"] },
    "credentials": ["apiKey"],
    "resources": { "memoryMiB": 64, "timeout": "PT30S", "maxConcurrent": 4 }
  },
  "rateLimits": [{ "name": "api", "key": "CREDENTIAL", "requestsPerSecond": 10, "burst": 20, "respectHeaders": { "retryAfter": "Retry-After" } }],
  "changelog": [{ "version": "VERSION", "date": "2026-10-01", "notes": ["changelog.orders"] }]
}`

type catalogFakePackages struct{ rows []connector.PackageRecord }

func (f catalogFakePackages) ListPackages(_ context.Context, after connector.PackageID, limit int) ([]connector.PackageRecord, error) {
	var out []connector.PackageRecord
	for i := range f.rows {
		if bytes.Compare(f.rows[i].ID[:], after[:]) > 0 && len(out) < limit {
			out = append(out, f.rows[i])
		}
	}
	return out, nil
}

func shopPackage(n byte, version string, status connector.PackageStatus) connector.PackageRecord {
	return connector.PackageRecord{
		ID: connector.PackageID{15: n}, ConnectorID: "acme.shop", Version: version, Status: status,
		Manifest: []byte(strings.ReplaceAll(catalogShopManifest, "VERSION", version)),
	}
}

func catalogTypes(t *testing.T, c *NodeCatalog, locale string) map[string]apigen.NodeType {
	t.Helper()
	rows, err := c.NodeTypes(WithLocale(t.Context(), locale), orgscope.Org{}, [16]byte{})
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]apigen.NodeType{}
	for i := range rows {
		if _, dup := out[rows[i].Type]; dup {
			t.Fatalf("type %s listed twice", rows[i].Type)
		}
		out[rows[i].Type] = rows[i]
	}
	return out
}

// TestBuiltinNodeTypesAreDescribedInTheLanguageOfTheRequest: the engine's own types come with
// translated texts, the safety of the HTTP node's operations, and the n8n types the importer maps.
func TestBuiltinNodeTypesAreDescribedInTheLanguageOfTheRequest(t *testing.T) {
	c := NewNodeCatalog(catalogFakePackages{}, nil)
	en := catalogTypes(t, c, LocaleEnglish)
	es := catalogTypes(t, c, LocaleSpanish)
	for _, typ := range []string{"orcon.http@1", "orcon.if@1", "orcon.switch@1", "orcon.loop@1", "orcon.set@1", "orcon.noop@1", "orcon.stopAndError@1", "orcon.filter@1", "orcon.sort@1", "orcon.limit@1"} {
		a, ok := en[typ]
		if !ok {
			t.Fatalf("%s is not in the catalog", typ)
		}
		b := es[typ]
		if a.Name == "" || a.Description == "" || b.Name == "" || b.Description == "" || a.Description == b.Description {
			t.Fatalf("%s: english %q/%q, spanish %q/%q", typ, a.Name, a.Description, b.Name, b.Description)
		}
		if a.Seal != apigen.NodeTypeSealOFFICIAL || a.ConnectorId != nil || a.Icon != nil || len(a.Operations) == 0 {
			t.Fatalf("%s = %+v", typ, a)
		}
	}
	http := en["orcon.http@1"]
	levels := map[string]apigen.NodeTypeOperationSafetyLevel{}
	for _, op := range http.Operations {
		levels[op.Name] = op.SafetyLevel
	}
	if levels["request.read"] != apigen.NONE || levels["request.write"] != apigen.UNSAFE {
		t.Fatalf("http safety = %v", levels)
	}
	for typ, n8n := range map[string]string{"orcon.filter@1": "n8n-nodes-base.filter", "orcon.sort@1": "n8n-nodes-base.sort", "orcon.limit@1": "n8n-nodes-base.limit"} {
		if got := en[typ].N8nTypes; !slices.Equal(got, []string{n8n}) {
			t.Errorf("%s maps n8n types %v, want [%s]", typ, got, n8n)
		}
	}
	if !slices.Contains(http.N8nTypes, "n8n-nodes-base.httpRequest") || !slices.Contains(en["orcon.if@1"].N8nTypes, "n8n-nodes-base.if") ||
		!slices.Contains(en["orcon.noop@1"].N8nTypes, "n8n-nodes-base.noOp") {
		t.Fatalf("n8n types = %v / %v", http.N8nTypes, en["orcon.if@1"].N8nTypes)
	}
}

// TestInstalledConnectorsJoinTheCatalog: the newest installed version of each connector shows its
// nodes, groups and operations with their safety; a retired version is left out; a package with no
// assets is not official and has no icon.
func TestInstalledConnectorsJoinTheCatalog(t *testing.T) {
	c := NewNodeCatalog(catalogFakePackages{rows: []connector.PackageRecord{
		shopPackage(1, "1.0.0", connector.StatusInstalled),
		shopPackage(2, "1.2.0", connector.StatusInstalled),
		shopPackage(3, "1.3.0", connector.StatusRetired),
	}}, nil)
	rows := catalogTypes(t, c, LocaleEnglish)
	order, ok := rows["acme.shop.order@1"]
	if !ok {
		t.Fatalf("catalog = %d types, the order node is missing", len(rows))
	}
	if order.Group != "Apps" || rows["acme.shop.orderCreated@1"].Group != "Triggers" || order.Seal != apigen.NodeTypeSealUNREVIEWED ||
		order.Icon != nil || order.ConnectorId == nil || order.ConnectorId.String() != id.UUID(connector.PackageID{15: 2}).String() {
		t.Fatalf("order = %+v", order)
	}
	if order.ProofLevel != "DECLARED" || !slices.Equal(order.N8nTypes, []string{"n8n-nodes-base.acmeShop"}) {
		t.Fatalf("proof level %s, n8n types %v", order.ProofLevel, order.N8nTypes)
	}
	levels := map[string]apigen.NodeTypeOperationSafetyLevel{}
	for _, op := range order.Operations {
		levels[op.Name] = op.SafetyLevel
	}
	want := map[string]apigen.NodeTypeOperationSafetyLevel{"order.get": apigen.NONE, "order.create": apigen.IDEMPOTENCYKEY, "order.send": apigen.UNSAFE}
	for name, level := range want {
		if levels[name] != level {
			t.Fatalf("safety of %s = %q, want %q (all: %v)", name, levels[name], level, levels)
		}
	}
}

const goodIcon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M12 2a10 10 0 1 0 0 20z"/></svg>`

func shopFiles(icon string) fstest.MapFS {
	return fstest.MapFS{
		"icons/icon.svg": {Data: []byte(icon)},
		"icons/SOURCE":   {Data: []byte("orcon-fallback\n")},
		"i18n/en.json":   {Data: []byte(`{"node.order.displayName":"Order","connector.description":"Shop orders."}`)},
		"i18n/es.json":   {Data: []byte(`{"node.order.displayName":"Pedido","connector.description":"Pedidos de la tienda.","operation.order.get.displayName":"Leer un pedido"}`)},
	}
}

// TestOfficialConnectorsCarryTheirIconAndTranslations: an official package shows the icon that passed
// the sanitizer, the seal OFFICIAL and the texts of its Spanish catalog; an unsafe icon never loads.
func TestOfficialConnectorsCarryTheirIconAndTranslations(t *testing.T) {
	assets, err := LoadConnectorAssets(shopFiles(goodIcon))
	if err != nil {
		t.Fatal(err)
	}
	c := NewNodeCatalog(catalogFakePackages{rows: []connector.PackageRecord{shopPackage(1, "1.0.0", connector.StatusInstalled)}},
		map[string]ConnectorAssets{"acme.shop": assets})
	es := catalogTypes(t, c, LocaleSpanish)["acme.shop.order@1"]
	if es.Name != "Pedido" || es.Description != "Pedidos de la tienda." || es.Seal != apigen.NodeTypeSealOFFICIAL {
		t.Fatalf("spanish order = %+v", es)
	}
	if es.Icon == nil || es.Icon.Svg != goodIcon || es.Icon.Source != "orcon-fallback" || es.Icon.DarkSvg != nil {
		t.Fatalf("icon = %+v", es.Icon)
	}
	if es.Operations[0].Label != "Leer un pedido" || es.Operations[1].Label != "Create an order" {
		t.Fatalf("operation labels = %q, %q (a missing translation keeps the manifest text)", es.Operations[0].Label, es.Operations[1].Label)
	}
	if en := catalogTypes(t, c, LocaleEnglish)["acme.shop.order@1"]; en.Name != "Order" || en.Description != "Shop orders." {
		t.Fatalf("english order = %+v", en)
	}

	_, err = LoadConnectorAssets(shopFiles(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`))
	if !errors.Is(err, authoring.ErrUnsafeIcon) {
		t.Fatalf("an unsafe icon loaded: %v", err)
	}
	if _, err := LoadConnectorAssets(fstest.MapFS{}); err == nil {
		t.Fatal("a package without an icon loaded")
	}
}

// TestNegotiateLocalePicksTheFirstLanguageTheAPISpeaks: Accept-Language decides between Spanish and
// English; anything else, or no header, is English.
func TestNegotiateLocalePicksTheFirstLanguageTheAPISpeaks(t *testing.T) {
	cases := map[string]string{
		"":                        LocaleEnglish,
		"es":                      LocaleSpanish,
		"es-CO,es;q=0.9,en;q=0.8": LocaleSpanish,
		"fr-FR, es;q=0.5":         LocaleSpanish,
		"en-US,en;q=0.9,es;q=0.8": LocaleEnglish,
		"fr-FR,de;q=0.7":          LocaleEnglish,
		";q=0.5,  ES-mx":          LocaleSpanish,
		"*":                       LocaleEnglish,
		"es-419;q=0.8, en;q=0.9":  LocaleSpanish,
		"zz\x00,\x01":             LocaleEnglish,
	}
	for header, want := range cases {
		if got := negotiateLocale(header); got != want {
			t.Errorf("negotiateLocale(%q) = %q, want %q", header, got, want)
		}
	}
	if LocaleFrom(t.Context()) != LocaleEnglish {
		t.Error("a context without a language is English")
	}
}
