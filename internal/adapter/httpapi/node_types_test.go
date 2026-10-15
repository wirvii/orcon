// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"testing"

	"github.com/wirvii/orcon/internal/adapter/httpapi/apigen"
	"github.com/wirvii/orcon/internal/adapter/httpapi/page"
	"github.com/wirvii/orcon/internal/authz"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
	"github.com/wirvii/orcon/internal/platform/orgscope/issue"
)

// fakeNodeTypes returns fixed node types, whatever the organization or project asks.
type fakeNodeTypes struct {
	rows []apigen.NodeType
}

func (f fakeNodeTypes) NodeTypes(context.Context, orgscope.Org, identity.ProjectID) ([]apigen.NodeType, error) {
	return f.rows, nil
}

func nodeTypeFixtures() []apigen.NodeType {
	return []apigen.NodeType{
		{
			Type: "orcon.whatsapp.whatsapp@1", Name: "WhatsApp Cloud API", Description: "Sends WhatsApp messages.",
			Group: "Connectors", Seal: "OFFICIAL", ProofLevel: "TESTED", N8nTypes: []string{},
			Operations: []apigen.NodeTypeOperation{},
			Icon: &apigen.NodeTypeIcon{
				Svg:    `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M1 1h22v22H1z"/></svg>`,
				Source: "simple-icons@16.35.0:whatsapp",
			},
		},
		{
			Type: "orcon.http@1", Name: "HTTP request", Description: "Calls an HTTP endpoint.",
			Group: "Core", Seal: "OFFICIAL", ProofLevel: "TESTED", N8nTypes: []string{},
			Operations: []apigen.NodeTypeOperation{},
		},
	}
}

// nodeTypesContext returns a context carrying a principal and an organization scope, as the
// pipeline leaves them for an authorized request.
func nodeTypesContext(t *testing.T) (context.Context, apigen.ProjectId) {
	t.Helper()
	ids := id.NewSequence()
	scope, err := issue.New(id.UUID(ids.New()))
	if err != nil {
		t.Fatal(err)
	}
	return authz.WithOrgScope(t.Context(), scope), apigen.ProjectId(ids.New())
}

// TestListNodeTypesIncludesIcon checks that a connector's type carries its icon inline and
// a built-in type carries none (SPEC-019 R-UX-BE-04, UX-I3).
func TestListNodeTypesIncludesIcon(t *testing.T) {
	ctx, pid := nodeTypesContext(t)
	h := NewNodeTypes(fakeNodeTypes{rows: nodeTypeFixtures()}, mustCodec(t))
	resp, err := h.ListNodeTypes(ctx, apigen.ListNodeTypesRequestObject{ProjectId: pid})
	if err != nil {
		t.Fatalf("list node types: %v", err)
	}
	body := resp.(apigen.ListNodeTypes200JSONResponse).Body
	if len(body.Items) != 2 {
		t.Fatalf("want 2 node types, got %d", len(body.Items))
	}
	byType := map[string]apigen.NodeType{}
	for _, nt := range body.Items {
		byType[nt.Type] = nt
	}
	wa := byType["orcon.whatsapp.whatsapp@1"]
	if wa.Icon == nil || wa.Icon.Svg == "" || wa.Icon.Source != "simple-icons@16.35.0:whatsapp" {
		t.Fatalf("the connector type must carry its icon inline, got %+v", wa.Icon)
	}
	if byType["orcon.http@1"].Icon != nil {
		t.Fatal("a built-in type without an icon must not carry one")
	}
}

// TestListNodeTypesFiltersAndPages checks the name filter and the page token.
func TestListNodeTypesFiltersAndPages(t *testing.T) {
	ctx, pid := nodeTypesContext(t)
	h := NewNodeTypes(fakeNodeTypes{rows: nodeTypeFixtures()}, mustCodec(t))
	q := "http"
	resp, err := h.ListNodeTypes(ctx, apigen.ListNodeTypesRequestObject{ProjectId: pid, Params: apigen.ListNodeTypesParams{Q: &q}})
	if err != nil {
		t.Fatal(err)
	}
	if items := resp.(apigen.ListNodeTypes200JSONResponse).Body.Items; len(items) != 1 || items[0].Type != "orcon.http@1" {
		t.Fatalf("the name filter must keep only HTTP request, got %+v", items)
	}

	one := 1
	first, err := h.ListNodeTypes(ctx, apigen.ListNodeTypesRequestObject{ProjectId: pid, Params: apigen.ListNodeTypesParams{Limit: &one}})
	if err != nil {
		t.Fatal(err)
	}
	firstBody := first.(apigen.ListNodeTypes200JSONResponse).Body
	if firstBody.NextPageToken == nil {
		t.Fatal("a second page must be announced with nextPageToken")
	}
	second, err := h.ListNodeTypes(ctx, apigen.ListNodeTypesRequestObject{ProjectId: pid, Params: apigen.ListNodeTypesParams{Limit: &one, PageToken: firstBody.NextPageToken}})
	if err != nil {
		t.Fatal(err)
	}
	secondBody := second.(apigen.ListNodeTypes200JSONResponse).Body
	if len(secondBody.Items) != 1 || secondBody.Items[0].Type == firstBody.Items[0].Type {
		t.Fatalf("the second page must hold the other type, got %+v", secondBody.Items)
	}
}

func mustCodec(t *testing.T) *page.Codec {
	t.Helper()
	codec, err := page.NewCodec(make([]byte, page.MinKeyLength))
	if err != nil {
		t.Fatal(err)
	}
	return codec
}
