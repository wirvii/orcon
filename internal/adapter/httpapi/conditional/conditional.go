// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package conditional implements optimistic concurrency with strong ETags
// (ADR-0053 section 5): a representation carries the revision it was read at,
// and a change must name that revision in If-Match. A missing If-Match is 428
// precondition-required; a revision that is no longer current is 412
// precondition-failed, with the current ETag so the client can read again.
package conditional

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/wirvii/orcon/internal/adapter/httpapi/problem"
)

// Errors of the package.
var (
	// ErrRequired means the request carries no If-Match.
	ErrRequired = errors.New("If-Match is required")
	// ErrMismatch means If-Match does not name the current revision.
	ErrMismatch = errors.New("If-Match does not match the current revision")
	// ErrMalformed means If-Match is not a strong ETag this API issued.
	ErrMalformed = errors.New("If-Match is not a strong ETag")
)

// ETag returns the strong entity tag of a revision: the revision in quotes, as in
// "7". Drafts use their draft revision; other resources their row revision.
func ETag(revision int64) string {
	return `"` + strconv.FormatInt(revision, 10) + `"`
}

// Parse reads a strong ETag issued by [ETag].
func Parse(tag string) (int64, error) {
	tag = strings.TrimSpace(tag)
	if len(tag) < 3 || tag[0] != '"' || tag[len(tag)-1] != '"' {
		return 0, ErrMalformed
	}
	n, err := strconv.ParseInt(tag[1:len(tag)-1], 10, 64)
	if err != nil || n < 0 {
		return 0, ErrMalformed
	}
	return n, nil
}

// Check compares the If-Match header of r with the current revision. It returns
// nil when they match, [ErrRequired] when the header is missing, and
// [ErrMismatch] when it names another revision or is not an ETag this API issued.
// The wildcard "*" never matches: a change is always based on a revision that was read.
func Check(r *http.Request, current int64) error {
	header := r.Header.Get("If-Match")
	if header == "" {
		return ErrRequired
	}
	got, err := Parse(header)
	if err != nil || got != current {
		return ErrMismatch
	}
	return nil
}

// Write answers a failed check with its problem: 428 for a missing If-Match, 412
// with the current ETag otherwise. Extra options add members, for example the
// current draft revision. It reports whether the check had failed.
func Write(w http.ResponseWriter, r *http.Request, err error, current int64, opts ...problem.Option) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, ErrRequired):
		problem.Write(w, r, problem.PreconditionRequired, opts...)
	default:
		opts = append([]problem.Option{problem.Header("ETag", ETag(current))}, opts...)
		problem.Write(w, r, problem.PreconditionFailed, opts...)
	}
	return true
}
