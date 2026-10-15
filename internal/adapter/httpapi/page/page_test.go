// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package page_test

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/adapter/httpapi/page"
)

var testKey = []byte("0123456789abcdef0123456789abcdef")

func codec(t *testing.T) *page.Codec {
	t.Helper()
	c, err := page.NewCodec(testKey)
	if err != nil {
		t.Fatalf("NewCodec: %v", err)
	}
	return c
}

func TestPageTokenRoundTrip(t *testing.T) {
	c := codec(t)
	b := page.Binding("status=ACTIVE", "sort=name")
	token, err := c.Encode("org-1", b, page.Cursor{Position: []string{"welcome", "0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a60"}})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	cur, err := c.Decode(token, "org-1", b)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if strings.Join(cur.Position, "|") != "welcome|0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a60" {
		t.Errorf("position = %v", cur.Position)
	}
	if strings.Contains(token, "welcome") {
		t.Log("the token is opaque to clients by convention only: its payload is signed, not encrypted")
	}
}

// TestPageTokenBoundToParams: a token reused with other filters, another sort or
// another organization is refused, and so is a forged or truncated one.
func TestPageTokenBoundToParams(t *testing.T) {
	c := codec(t)
	b := page.Binding("status=ACTIVE", "sort=name")
	token, _ := c.Encode("org-1", b, page.Cursor{Position: []string{"a", "1"}})
	other, _ := page.NewCodec([]byte("ffffffffffffffffffffffffffffffff"))
	body, mac, _ := strings.Cut(token, ".")
	forged := body + "." + strings.Repeat("A", len(mac))
	tests := map[string]struct {
		codec          *page.Codec
		token, org, bd string
	}{
		"other filters":      {c, token, "org-1", page.Binding("status=INACTIVE", "sort=name")},
		"other sort":         {c, token, "org-1", page.Binding("status=ACTIVE", "sort=-name")},
		"other organization": {c, token, "org-2", b},
		"other key":          {other, token, "org-1", b},
		"forged mac":         {c, forged, "org-1", b},
		"no separator":       {c, "garbage", "org-1", b},
		"bad base64 body":    {c, "!!!." + mac, "org-1", b},
		"bad base64 mac":     {c, body + ".!!!", "org-1", b},
		"empty":              {c, "", "org-1", b},
		"body not JSON":      {c, "e30." + mac, "org-1", b},
	}
	for name, tt := range tests {
		if _, err := tt.codec.Decode(tt.token, tt.org, tt.bd); !errors.Is(err, page.ErrInvalidToken) {
			t.Errorf("%s: err = %v, want ErrInvalidToken", name, err)
		}
	}
	if _, err := c.Decode(token, "org-1", b); err != nil {
		t.Errorf("the token is refused for its own parameters: %v", err)
	}
}

func TestNewCodecRejectsShortKeys(t *testing.T) {
	if _, err := page.NewCodec([]byte("short")); !errors.Is(err, page.ErrKeyTooShort) {
		t.Errorf("err = %v, want ErrKeyTooShort", err)
	}
}

func TestLimit(t *testing.T) {
	for raw, want := range map[string]int{"": 50, "1": 1, "50": 50, "200": 200} {
		if got, err := page.Limit(raw); err != nil || got != want {
			t.Errorf("Limit(%q) = %d, %v; want %d", raw, got, err, want)
		}
	}
	for _, raw := range []string{"0", "201", "-1", "abc", "1.5"} {
		if _, err := page.Limit(raw); !errors.Is(err, page.ErrInvalidLimit) {
			t.Errorf("Limit(%q) err = %v, want ErrInvalidLimit", raw, err)
		}
	}
}

// TestSortAllowlist: a field outside the allowlist, a repeated field or an empty
// term is refused; the default applies when nothing is sent.
func TestSortAllowlist(t *testing.T) {
	allowed := []string{"name", "-name", "updatedAt", "-updatedAt"}
	terms, err := page.ParseSort("name,-updatedAt", allowed, "-updatedAt")
	if err != nil || page.String(terms) != "name,-updatedAt" {
		t.Fatalf("ParseSort = %v, %v", terms, err)
	}
	if terms, err = page.ParseSort("", allowed, "-updatedAt"); err != nil || page.String(terms) != "-updatedAt" {
		t.Errorf("the default = %v, %v", terms, err)
	}
	for _, raw := range []string{"secret", "name,secret", "name,name", "name,-name", ",", "name,", "--name", "createdAt"} {
		if _, err := page.ParseSort(raw, allowed, "name"); !errors.Is(err, page.ErrInvalidSort) {
			t.Errorf("ParseSort(%q) err = %v, want ErrInvalidSort", raw, err)
		}
	}
}

// row is an item of the fake collection; (Name, ID) is the total order.
type row struct{ Name, ID string }

func (a row) less(b row) bool {
	if a.Name != b.Name {
		return a.Name < b.Name
	}
	return a.ID < b.ID
}

// collection is the keyset store the real adapters implement with a WHERE clause.
type collection struct{ rows []row }

func (c *collection) insert(r row) {
	c.rows = append(c.rows, r)
	sort.Slice(c.rows, func(i, j int) bool { return c.rows[i].less(c.rows[j]) })
}

// after returns up to limit rows strictly after the position, in order.
func (c *collection) after(pos []string, limit int) []row {
	var out []row
	for _, r := range c.rows {
		if len(pos) == 2 && !(row{pos[0], pos[1]}).less(r) {
			continue
		}
		out = append(out, r)
		if len(out) == limit {
			break
		}
	}
	return out
}

// TestPaginationStableUnderInserts: walking a collection while rows are inserted
// neither repeats nor skips a row that existed when the walk began.
func TestPaginationStableUnderInserts(t *testing.T) {
	c := codec(t)
	coll := &collection{}
	original := map[string]bool{}
	for i := range 40 {
		r := row{Name: fmt.Sprintf("flow-%02d", i*2), ID: fmt.Sprintf("id-%03d", i)}
		coll.insert(r)
		original[r.ID] = true
	}
	binding := page.Binding("sort=name")
	seen := map[string]int{}
	token := ""
	for pageNumber := 0; ; pageNumber++ {
		var pos []string
		if token != "" {
			cur, err := c.Decode(token, "org-1", binding)
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			pos = cur.Position
		}
		rows := coll.after(pos, 7)
		for _, r := range rows {
			seen[r.ID]++
		}
		// Between two pages: rows land before the cursor, right at it and after it.
		coll.insert(row{Name: "flow-00", ID: fmt.Sprintf("early-%d", pageNumber)})
		coll.insert(row{Name: rows[len(rows)-1].Name, ID: fmt.Sprintf("tie-%d", pageNumber)})
		coll.insert(row{Name: "flow-99", ID: fmt.Sprintf("late-%d", pageNumber)})
		if len(rows) < 7 {
			break
		}
		last := rows[len(rows)-1]
		var err error
		if token, err = c.Encode("org-1", binding, page.Cursor{Position: []string{last.Name, last.ID}}); err != nil {
			t.Fatalf("Encode: %v", err)
		}
	}
	for id := range original {
		if seen[id] != 1 {
			t.Errorf("row %s was returned %d times, want exactly once", id, seen[id])
		}
	}
	for id, n := range seen {
		if n > 1 {
			t.Errorf("row %s was returned %d times", id, n)
		}
	}
}
