// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Severity tells whether an [Issue] blocks publication.
type Severity string

// Severities (ADR-0007 section 9: errors block publication; warnings are
// shown and stored with the version).
const (
	// SeverityError blocks publication.
	SeverityError Severity = "ERROR"
	// SeverityWarning is shown and stored with the version.
	SeverityWarning Severity = "WARNING"
)

// Issue codes. V1..V13 are the static rules of ADR-0010 section 11; the
// others group the checks of the remaining validation levels.
const (
	// CodeStructure marks a document that does not have the orcon/flow@1 shape.
	CodeStructure = "STRUCTURE"
	// CodeFormat marks a wrong or unsupported format tag.
	CodeFormat = "FORMAT"
	// CodeEnum marks a value outside a closed UPPER_SNAKE value set.
	CodeEnum = "ENUM"
	// CodeValue marks a value that is out of range or badly formed.
	CodeValue = "VALUE"
	// CodeExt marks an extension member that is not namespaced.
	CodeExt = "EXT"
	// CodeReference marks an id, name or port that points at nothing.
	CodeReference = "REFERENCE"
	// CodeBlock marks a behavior block used on a node that cannot have it.
	CodeBlock = "BLOCK"
	// CodeAttention marks an attention notification that cannot be turned off.
	CodeAttention = "ATTENTION"
	// CodeConnector marks a node whose connector pin is out of place (on a type
	// no package owns) or that level 5 cannot accept: the version is not
	// installed, not granted, or blocked by the policy.
	CodeConnector = "CONNECTOR"
	// CodeV1 is rule V1: every node is reachable from a start event.
	CodeV1 = "V1"
	// CodeV2 is rule V2: every cycle has a loop control and back edges target one.
	CodeV2 = "V2"
	// CodeV3 is rule V3: a loop control sets maxIterations.
	CodeV3 = "V3"
	// CodeV4 is rule V4: signal and human waits have a timeout path.
	CodeV4 = "V4"
	// CodeV5 is rule V5: exclusive and inclusive splits have a default.
	CodeV5 = "V5"
	// CodeV6 is rule V6: a join has at least two incoming edges.
	CodeV6 = "V6"
	// CodeV7 is rule V7: boundary events attach to activities only.
	CodeV7 = "V7"
	// CodeV8 is rule V8: a node reads only the outputs of its ancestors.
	CodeV8 = "V8"
	// CodeV9 is rule V9: a plain variable has no concurrent writers.
	CodeV9 = "V9"
	// CodeV10 is rule V10: effect nodes carry a valid safety contract.
	CodeV10 = "V10"
	// CodeV11 is rule V11: the call graph has no unmarked cycle.
	CodeV11 = "V11"
	// CodeV12 is rule V12: node names are unique.
	CodeV12 = "V12"
	// CodeV13 is rule V13, a warning: the condition that ends a loop reads
	// nothing the loop body writes.
	CodeV13 = "V13"
)

// Issue is one finding of a validation.
type Issue struct {
	Severity Severity
	// Code names the rule, for example "V8" or "ENUM".
	Code string
	// Path is a JSON Pointer (RFC 6901) to the offending part of the document.
	Path    string
	Message string
}

// String renders the issue on one line.
func (i Issue) String() string {
	return fmt.Sprintf("%s %s at %s: %s", i.Severity, i.Code, i.Path, i.Message)
}

// Report is the outcome of a validation.
type Report struct {
	Issues []Issue
}

// HasErrors reports whether any issue blocks publication.
func (r Report) HasErrors() bool {
	return slices.ContainsFunc(r.Issues, func(i Issue) bool { return i.Severity == SeverityError })
}

// Errors returns the issues that block publication.
func (r Report) Errors() []Issue { return r.bySeverity(SeverityError) }

// Warnings returns the issues that do not block publication.
func (r Report) Warnings() []Issue { return r.bySeverity(SeverityWarning) }

// Has reports whether the report holds an issue with the given code and severity.
func (r Report) Has(code string, severity Severity) bool {
	return slices.ContainsFunc(r.Issues, func(i Issue) bool { return i.Code == code && i.Severity == severity })
}

// Err returns a [*ValidationError] when the report has errors, and nil otherwise.
func (r Report) Err() error {
	if !r.HasErrors() {
		return nil
	}
	return &ValidationError{Report: r}
}

func (r Report) bySeverity(s Severity) []Issue {
	var out []Issue
	for _, i := range r.Issues {
		if i.Severity == s {
			out = append(out, i)
		}
	}
	return out
}

// collector accumulates the issues of one validation run.
type collector struct {
	issues []Issue
}

func (c *collector) addf(severity Severity, code, path, format string, args ...any) {
	c.issues = append(c.issues, Issue{Severity: severity, Code: code, Path: path, Message: fmt.Sprintf(format, args...)})
}

func (c *collector) errorf(code, path, format string, args ...any) {
	c.addf(SeverityError, code, path, format, args...)
}

func (c *collector) warnf(code, path, format string, args ...any) {
	c.addf(SeverityWarning, code, path, format, args...)
}

func (c *collector) hasErrors() bool {
	return slices.ContainsFunc(c.issues, func(i Issue) bool { return i.Severity == SeverityError })
}

// report returns the issues ordered by path, then code and message, so
// reports are stable.
func (c *collector) report() Report {
	issues := slices.Clone(c.issues)
	slices.SortStableFunc(issues, func(a, b Issue) int {
		return strings.Compare(a.Path+"\x00"+a.Code+"\x00"+a.Message, b.Path+"\x00"+b.Code+"\x00"+b.Message)
	})
	return Report{Issues: issues}
}

// ValidationError is returned when a document has validation errors.
type ValidationError struct {
	Report Report
}

// Error summarizes the first error and the number of others.
func (e *ValidationError) Error() string {
	errs := e.Report.Errors()
	if len(errs) == 0 {
		return "flow validation failed"
	}
	if len(errs) == 1 {
		return "flow validation failed: " + errs[0].String()
	}
	return fmt.Sprintf("flow validation failed: %s (and %d more errors)", errs[0], len(errs)-1)
}

// Sentinel errors of the package.
var (
	// ErrDraftConflict means the draft changed since the caller read it
	// (optimistic concurrency on draftRevision).
	ErrDraftConflict = errors.New("flow draft was changed by someone else")
	// ErrVersionNotFound means the requested flow version does not exist.
	ErrVersionNotFound = errors.New("flow version not found")
	// ErrSnapshotNotFound means the requested snapshot does not exist.
	ErrSnapshotNotFound = errors.New("flow snapshot not found")
	// ErrInvalidRevision means a revision does not pin exactly one of a
	// version or a snapshot.
	ErrInvalidRevision = errors.New("invalid flow revision")
)

// UnknownFormatError reports a document whose format this build cannot read:
// a malformed tag, an unknown family, or a major version newer than the build
// supports (ADR-0007 section 7).
type UnknownFormatError struct {
	// Format is the tag found in the document, as written.
	Format string
	// Oldest and Newest are the supported major versions.
	Oldest, Newest int
}

// Error names the format found and the range this build supports.
func (e *UnknownFormatError) Error() string {
	return fmt.Sprintf("unknown flow format %q: this build supports %s%d through %s%d",
		e.Format, formatPrefix, e.Oldest, formatPrefix, e.Newest)
}
