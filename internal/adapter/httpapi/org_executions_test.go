// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/oapi-codegen/runtime/types"

	"github.com/wirvii/orcon/internal/adapter/httpapi/apigen"
	"github.com/wirvii/orcon/internal/adapter/httpapi/page"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// executionsFake is the read side of executions of a test: the executions of each project in memory,
// listed the way the stores list them (by creation instant and identity, in the direction asked for, past
// the cursor), and the size of every read recorded.
type executionsFake struct {
	byProject map[identity.ProjectID][]store.Execution
	limits    []int
}

func (f *executionsFake) ListProjectExecutions(_ context.Context, _ orgscope.Org, project identity.ProjectID, query store.ExecutionQuery, cursor store.Cursor, limit int) ([]store.Execution, error) {
	f.limits = append(f.limits, limit)
	rows := slices.Clone(f.byProject[project])
	slices.SortFunc(rows, func(a, b store.Execution) int {
		if c := a.CreatedAt.Compare(b.CreatedAt); c != 0 {
			return c
		}
		return slices.Compare(a.ID[:], b.ID[:])
	})
	if !query.OldestFirst {
		slices.Reverse(rows)
	}
	var out []store.Execution
	for i := range rows {
		e := &rows[i]
		if !cursor.IsZero() {
			c := e.CreatedAt.Compare(cursor.CreatedAt)
			if c == 0 {
				c = slices.Compare(e.ID[:], cursor.ID[:])
			}
			if (query.OldestFirst && c <= 0) || (!query.OldestFirst && c >= 0) {
				continue
			}
		}
		out = append(out, *e)
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

// TestListOrgExecutionsPagesEveryExecutionInOrder: 1500 executions of three projects, many of them with the
// same creation instant, are listed page by page in both sort directions. Every execution appears exactly
// once, in the order asked for; each read asks a project for one row more than a page; a page token is
// present until the last execution is served and absent after it (BL-135).
func TestListOrgExecutionsPagesEveryExecutionInOrder(t *testing.T) {
	r := newProjectsRig(t)
	ids := id.NewSequence()
	reader := &executionsFake{byProject: map[identity.ProjectID][]store.Execution{}}
	var projects []identity.ProjectID
	for _, name := range []string{"One", "Two", "Three"} {
		p, err := r.createProject(name, "")
		if err != nil {
			t.Fatal(err)
		}
		projects = append(projects, identity.ProjectID(*p.Body.Id))
	}
	start := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	const total = 1500
	for i := range total {
		p := projects[i%len(projects)]
		// Three executions in every instant, one per project, so the identity breaks the ties.
		at := start.Add(time.Duration(i/3) * time.Second)
		e := store.Execution{ID: store.ExecutionID(ids.New()), OrgID: r.org, ProjectID: p, Status: "SUCCEEDED", Mode: "TRIGGER", CreatedAt: at}
		reader.byProject[p] = append(reader.byProject[p], e)
	}
	codec, err := page.NewCodec(make([]byte, page.MinKeyLength))
	if err != nil {
		t.Fatal(err)
	}
	h := NewOrgExecutions(r.access, r.grants, reader, codec)

	for _, sort := range []apigen.ListOrgExecutionsParamsSort{apigen.ListOrgExecutionsParamsSortCreatedAt, apigen.ListOrgExecutionsParamsSortMinusCreatedAt} {
		t.Run(string(sort), func(t *testing.T) {
			reader.limits = nil
			const limit = 100
			var seen []apigen.Execution
			var token *string
			pages := 0
			for {
				pageSize := limit
				resp, err := h.ListOrgExecutions(r.ctx, apigen.ListOrgExecutionsRequestObject{
					OrgId: types.UUID(r.org), Params: apigen.ListOrgExecutionsParams{Limit: &pageSize, PageToken: token, Sort: &sort},
				})
				if err != nil {
					t.Fatalf("page %d: %v", pages+1, err)
				}
				ok, isList := resp.(apigen.ListOrgExecutions200JSONResponse)
				if !isList {
					t.Fatalf("page %d answered %T, want 200", pages+1, resp)
				}
				body := ok.Body
				pages++
				seen = append(seen, body.Items...)
				if body.NextPageToken == nil {
					break
				}
				if len(seen) >= total {
					t.Fatalf("page %d: a page token follows the last execution", pages)
				}
				token = body.NextPageToken
				if pages > total/limit+1 {
					t.Fatal("the pages do not end")
				}
			}
			if len(seen) != total || pages != total/limit {
				t.Fatalf("served %d executions in %d pages, want %d in %d", len(seen), pages, total, total/limit)
			}
			unique := map[types.UUID]bool{}
			for i, e := range seen {
				if unique[*e.Id] {
					t.Fatalf("execution %s served twice", e.Id)
				}
				unique[*e.Id] = true
				if i == 0 {
					continue
				}
				prev := seen[i-1]
				c := prev.CreatedAt.Compare(e.CreatedAt)
				if c == 0 {
					c = slices.Compare(prev.Id[:], e.Id[:])
				}
				if (sort == apigen.ListOrgExecutionsParamsSortCreatedAt && c >= 0) || (sort == apigen.ListOrgExecutionsParamsSortMinusCreatedAt && c <= 0) {
					t.Fatalf("position %d is out of order for sort %s: %s then %s", i, sort, prev.CreatedAt, e.CreatedAt)
				}
			}
			for _, l := range reader.limits {
				if l != limit+1 {
					t.Fatalf("a project read asked for %d rows, want limit+1 = %d", l, limit+1)
				}
			}
		})
	}
}
