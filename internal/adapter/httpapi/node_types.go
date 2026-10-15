// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"strings"

	"github.com/wirvii/orcon/internal/adapter/httpapi/apigen"
	"github.com/wirvii/orcon/internal/adapter/httpapi/page"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// nodeTypeSortFields are the keys the node type listing orders by; the listing has
// one fixed order, by type identifier.
var nodeTypeSortFields = []string{"type"}

// NodeTypeCatalog lists the node types a project can use: the built-in ones and those
// of the installed connectors. Each entry carries its icon inline when the connector
// ships one (SPEC-019 R-UX-BE-04); a type without one has no icon field. The connector
// registry serves the port.
type NodeTypeCatalog interface {
	NodeTypes(ctx context.Context, org orgscope.Org, project identity.ProjectID) ([]apigen.NodeType, error)
}

// NodeTypes serves listNodeTypes. It is not routed yet: no catalog is wired at the
// composition root, so the operation keeps answering 503 from Unavailable.
type NodeTypes struct {
	catalog NodeTypeCatalog
	codec   *page.Codec
}

// NewNodeTypes returns the handler over the catalog and the page token codec.
func NewNodeTypes(catalog NodeTypeCatalog, codec *page.Codec) *NodeTypes {
	return &NodeTypes{catalog: catalog, codec: codec}
}

// ListNodeTypes lists the node types of a project, optionally filtered by a name
// fragment (q, case-insensitive), a palette group and a connector, one page at a time.
// Permission project:read is checked by the pipeline before this runs.
func (n *NodeTypes) ListNodeTypes(ctx context.Context, req apigen.ListNodeTypesRequestObject) (apigen.ListNodeTypesResponseObject, error) { //nolint:ireturn // the generated interface requires it
	org, err := scopeOf(ctx)
	if err != nil {
		return nil, err
	}
	limit, err := pageLimit(req.Params.Limit)
	if err != nil {
		return nil, err
	}
	terms, err := page.ParseSort("", nodeTypeSortFields, "type")
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(derefString(req.Params.Q))
	group := derefString(req.Params.Group)
	connector := ""
	if req.Params.ConnectorId != nil {
		connector = req.Params.ConnectorId.String()
	}
	project := identity.ProjectID(req.ProjectId)
	binding := page.Binding("q="+q, "group="+group, "connector="+connector, "project="+project.String())
	cursor, err := cursorFor(n.codec, derefString(req.Params.PageToken), org.String(), binding, terms)
	if err != nil {
		return nil, err
	}

	rows, err := n.catalog.NodeTypes(ctx, org, project)
	if err != nil {
		return nil, err
	}
	var items []listItem
	for i := range rows {
		row := &rows[i]
		if q != "" && !strings.Contains(strings.ToLower(row.Name), q) {
			continue
		}
		if group != "" && row.Group != group {
			continue
		}
		if connector != "" && (row.ConnectorId == nil || row.ConnectorId.String() != connector) {
			continue
		}
		items = append(items, listItem{keys: map[string]string{"type": row.Type}, id: row.Type, idx: i})
	}
	window, more, last := listWindow(items, terms, limit, cursor)

	body := apigen.NodeTypeList{Items: []apigen.NodeType{}}
	for _, it := range window {
		body.Items = append(body.Items, rows[it.idx])
	}
	if more {
		token, err := tokenFor(n.codec, org.String(), binding, last)
		if err != nil {
			return nil, err
		}
		body.NextPageToken = &token
	}
	return apigen.ListNodeTypes200JSONResponse{Body: body}, nil
}
