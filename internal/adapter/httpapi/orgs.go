// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"slices"
	"strings"

	"github.com/oapi-codegen/runtime/types"

	"github.com/wirvii/orcon/internal/adapter/httpapi/apigen"
	"github.com/wirvii/orcon/internal/adapter/httpapi/middleware"
	"github.com/wirvii/orcon/internal/identity"
)

// errInvalidPageToken is what a listing answers for a page token it did not issue
// for this query; the error handler turns it into invalid-page-token.
var errInvalidPageToken = errors.New("invalid page token")

// Page size bounds of the listings (SPEC-016 R-SH-API-03).
const (
	defaultPageSize = 50
	maxPageSize     = 200
)

// Orgs serves listOrgs (SPEC-016 section 3.1, R-SH-ORG-03). It is not routed yet:
// the operation keeps answering 503 from Unavailable until the composition root wires
// the session authenticator that gives it a user.
type Orgs struct {
	resolver identity.OrganizationResolver
}

// NewOrgs returns the handler over the organization resolver.
func NewOrgs(resolver identity.OrganizationResolver) *Orgs {
	return &Orgs{resolver: resolver}
}

// ListOrgs lists the organizations of the signed-in user, by name and then id, one
// page at a time. The slug filter is exact. The list reads at most
// identity.MaxResolvedOrganizations organizations, so a user with more sees the first
// ones by name.
func (o *Orgs) ListOrgs(ctx context.Context, req apigen.ListOrgsRequestObject) (apigen.ListOrgsResponseObject, error) { //nolint:ireturn // the generated interface requires it
	principal, ok := middleware.PrincipalFrom(ctx)
	if !ok || principal.UserID.IsZero() {
		return nil, identity.ErrUnauthenticated
	}
	limit := defaultPageSize
	if req.Params.Limit != nil {
		limit = *req.Params.Limit
	}
	if limit < 1 || limit > maxPageSize {
		return nil, identity.ErrInvalidInput
	}
	slug := ""
	if req.Params.Slug != nil {
		slug = *req.Params.Slug
	}
	query := orgQuery{Slug: slug, Limit: limit}
	var after *orgCursor
	if req.Params.PageToken != nil {
		c, err := decodeOrgCursor(*req.Params.PageToken, query)
		if err != nil {
			return nil, err
		}
		after = &c
	}

	all, err := o.resolver.ResolveUserOrganizations(ctx, principal.UserID, identity.MaxResolvedOrganizations)
	if err != nil {
		return nil, err
	}
	// The order is the contract's, whatever the collation of the database is.
	slices.SortFunc(all, func(a, b identity.OrganizationChoice) int {
		if c := strings.Compare(a.Name, b.Name); c != 0 {
			return c
		}
		return bytes.Compare(a.ID[:], b.ID[:])
	})

	var page []identity.OrganizationChoice
	for _, org := range all {
		if slug != "" && org.Slug != slug {
			continue
		}
		if after != nil && !after.before(org) {
			continue
		}
		page = append(page, org)
		if len(page) > limit {
			break
		}
	}

	body := apigen.OrganizationList{Items: []apigen.Organization{}}
	more := len(page) > limit
	if more {
		page = page[:limit]
	}
	for _, org := range page {
		oid := types.UUID(org.ID)
		body.Items = append(body.Items, apigen.Organization{Id: &oid, Slug: org.Slug, Name: org.Name})
	}
	if more {
		token, err := encodeOrgCursor(page[len(page)-1], query)
		if err != nil {
			return nil, err
		}
		body.NextPageToken = &token
	}
	return apigen.ListOrgs200JSONResponse{Body: body}, nil
}

// orgQuery is what a page token is bound to: a token of another query is refused.
type orgQuery struct {
	Slug  string `json:"s"`
	Limit int    `json:"l"`
}

// orgCursor is the position of the last organization of a page: (name, id).
type orgCursor struct {
	Name  string   `json:"n"`
	ID    string   `json:"i"`
	Query orgQuery `json:"q"`
}

// before reports whether org comes after the cursor in the (name, id) order.
func (c orgCursor) before(org identity.OrganizationChoice) bool {
	if org.Name != c.Name {
		return org.Name > c.Name
	}
	return org.ID.String() > c.ID
}

// encodeOrgCursor returns the opaque token of the position after org.
func encodeOrgCursor(org identity.OrganizationChoice, q orgQuery) (string, error) {
	raw, err := json.Marshal(orgCursor{Name: org.Name, ID: org.ID.String(), Query: q})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// decodeOrgCursor reads a page token. A token that does not decode, or that was issued
// for another filter or page size, is errInvalidPageToken.
func decodeOrgCursor(token string, q orgQuery) (orgCursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return orgCursor{}, errInvalidPageToken
	}
	var c orgCursor
	if err := json.Unmarshal(raw, &c); err != nil || c.Query != q || c.ID == "" {
		return orgCursor{}, errInvalidPageToken
	}
	return c, nil
}
