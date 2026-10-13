// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// UpgradeStep turns a document of major version N into one of version N+1.
// It must be pure and total: no I/O, no clock, no randomness, and an answer
// (a document or an error) for every input. It must not set the "format"
// member; the [Upgrader] does that after the step succeeds.
type UpgradeStep func(jsonvalue.Value) (jsonvalue.Value, error)

// Upgrader reads documents of older formats by running a chain of
// [UpgradeStep] values (ADR-0007 section 7). It never rewrites anything that
// is stored: it returns a new value and leaves its input alone.
type Upgrader struct {
	current int
	steps   map[int]UpgradeStep // steps[n] upgrades major n to n+1
}

// NewUpgrader returns an Upgrader whose newest supported major version is
// current. The steps map must hold one step for every version from 1 to current-1.
func NewUpgrader(current int, steps map[int]UpgradeStep) (*Upgrader, error) {
	if current < 1 {
		return nil, fmt.Errorf("new flow upgrader: current major %d is below 1", current)
	}
	for n := 1; n < current; n++ {
		if steps[n] == nil {
			return nil, fmt.Errorf("new flow upgrader: no step from %s%d to %s%d", formatPrefix, n, formatPrefix, n+1)
		}
	}
	for n := range steps {
		if n < 1 || n >= current {
			return nil, fmt.Errorf("new flow upgrader: step from major %d is outside 1..%d", n, current-1)
		}
	}
	copied := make(map[int]UpgradeStep, len(steps))
	for n, s := range steps {
		copied[n] = s
	}
	return &Upgrader{current: current, steps: copied}, nil
}

// Current returns the newest major version the upgrader produces.
func (u *Upgrader) Current() int { return u.current }

// Upgrade brings a document to the newest major version. A document already
// at that version is returned as is. A document of an unknown or newer format
// is rejected with an [*UnknownFormatError] naming the format and the range
// this build supports.
func (u *Upgrader) Upgrade(v jsonvalue.Value) (jsonvalue.Value, error) {
	tag, ok := v.Get("format")
	if !ok {
		return jsonvalue.Value{}, &ValidationError{Report: Report{Issues: []Issue{{
			Severity: SeverityError, Code: CodeFormat, Path: "/format", Message: "missing required property \"format\"",
		}}}}
	}
	text, _ := tag.Str()
	major, ok := parseFormat(text)
	if !ok || major < 1 || major > u.current {
		return jsonvalue.Value{}, &UnknownFormatError{Format: text, Oldest: 1, Newest: u.current}
	}
	for n := major; n < u.current; n++ {
		next, err := u.steps[n](v)
		if err != nil {
			return jsonvalue.Value{}, fmt.Errorf("upgrade flow from %s%d to %s%d: %w", formatPrefix, n, formatPrefix, n+1, err)
		}
		if next.Kind() != jsonvalue.KindObject {
			return jsonvalue.Value{}, fmt.Errorf("upgrade flow from %s%d to %s%d: the step returned %s, not an object", formatPrefix, n, formatPrefix, n+1, next.Kind())
		}
		v = next.With("format", jsonvalue.String(formatPrefix+strconv.Itoa(n+1)))
	}
	return v, nil
}

// defaultUpgrader knows orcon/flow@1 only; the chain grows with each major.
func defaultUpgrader() *Upgrader {
	u, err := NewUpgrader(FormatMajor, nil)
	if err != nil {
		panic("flow: default upgrader is inconsistent: " + err.Error())
	}
	return u
}

// Upgrade brings a document to the format of this build with the built-in
// upgrade chain. See [Upgrader.Upgrade].
func Upgrade(v jsonvalue.Value) (jsonvalue.Value, error) {
	return defaultUpgrader().Upgrade(v)
}

// parseFormat extracts N from "orcon/flow@N".
func parseFormat(tag string) (int, bool) {
	rest, ok := strings.CutPrefix(tag, formatPrefix)
	if !ok || rest == "" || rest[0] == '0' || rest[0] == '+' || rest[0] == '-' {
		return 0, false
	}
	n, err := strconv.Atoi(rest)
	if err != nil {
		return 0, false
	}
	return n, true
}
