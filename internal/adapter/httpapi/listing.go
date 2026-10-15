// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/wirvii/orcon/internal/adapter/httpapi/page"
)

// maxListRows bounds the rows one listing reads before it filters, sorts and pages them. A
// listing beyond it sees the first rows in identity order, as the organization listing does
// (SPEC-016 R-SH-API-03).
const maxListRows = 1000

// listItem is what the keyset walk needs of a listed row: the value of each sort field, as text
// that sorts the way the field does, and the identity.
type listItem struct {
	keys map[string]string
	id   string
	// idx is the position of the row in the slice the walk was given.
	idx int
}

// sortKeyTime is the text of an instant that sorts in time order.
func sortKeyTime(t time.Time) string { return fmt.Sprintf("%020d", t.UnixMicro()) }

// position returns the values that place an item in the order of terms: its sort keys, then its
// identity, which makes the order total.
func (it listItem) position(terms []page.Term) []string {
	out := make([]string, 0, len(terms)+1)
	for _, t := range terms {
		out = append(out, it.keys[t.Field])
	}
	return append(out, it.id)
}

// comparePositions orders two positions by terms (a leading minus reverses a field), then by identity.
func comparePositions(a, b []string, terms []page.Term) int {
	for i, t := range terms {
		c := strings.Compare(a[i], b[i])
		if c != 0 {
			if t.Descending {
				return -c
			}
			return c
		}
	}
	return strings.Compare(a[len(terms)], b[len(terms)])
}

// listWindow sorts items by terms, keeps those after the cursor position (nil means the first
// page), and returns at most limit of them with the position of the last one when more remain.
func listWindow(items []listItem, terms []page.Term, limit int, cursor []string) (window []listItem, more bool, last []string) {
	slices.SortFunc(items, func(a, b listItem) int {
		return comparePositions(a.position(terms), b.position(terms), terms)
	})
	for _, it := range items {
		if cursor != nil && comparePositions(it.position(terms), cursor, terms) <= 0 {
			continue
		}
		window = append(window, it)
		if len(window) > limit {
			break
		}
	}
	if len(window) > limit {
		window = window[:limit]
		more = true
		last = window[len(window)-1].position(terms)
	}
	return window, more, last
}

// cursorFor decodes the position of a page token. A token that does not belong to this
// organization, query and sort is page.ErrInvalidToken.
func cursorFor(codec *page.Codec, token, org, binding string, terms []page.Term) ([]string, error) {
	if token == "" {
		return nil, nil
	}
	cur, err := codec.Decode(token, org, binding)
	if err != nil {
		return nil, err
	}
	if len(cur.Position) != len(terms)+1 {
		return nil, page.ErrInvalidToken
	}
	return cur.Position, nil
}

// tokenFor encodes the position after the last item of a page.
func tokenFor(codec *page.Codec, org, binding string, last []string) (string, error) {
	return codec.Encode(org, binding, page.Cursor{Position: last})
}
