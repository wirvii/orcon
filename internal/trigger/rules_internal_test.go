// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package trigger

import (
	"strings"
	"testing"
)

func TestValidateCron(t *testing.T) {
	t.Parallel()
	good := []string{
		"* * * * *", "0 9 * * 1-5", "*/15 * * * *", "0 0 1 JAN *", "0 0 * * SUN", "30 8 * * MON,WED,FRI",
		"0 0 9 * * *", "10-20/2 * * * *", "0 0 * * 7",
	}
	for _, expr := range good {
		if err := validateCron(expr); err != nil {
			t.Errorf("validateCron(%q) = %v, want it valid", expr, err)
		}
	}
	bad := map[string]string{
		"":              "fields",
		"* * *":         "fields",
		"* * * * * * *": "fields",
		"60 * * * *":    "minutes",
		"* 24 * * *":    "hours",
		"* * 0 * *":     "day of month",
		"* * * 13 *":    "month",
		"* * * * 8":     "day of week",
		"*/0 * * * *":   "step",
		"*/x * * * *":   "step",
		"5-1 * * * *":   "backwards",
		"a * * * *":     "minutes",
		"* * * FOO *":   "month",
	}
	for expr, field := range bad {
		err := validateCron(expr)
		if err == nil {
			t.Errorf("validateCron(%q) accepted a bad expression", expr)
			continue
		}
		if msg := err.Error(); !strings.Contains(msg, field) {
			t.Errorf("validateCron(%q) = %q, want it to mention %q", expr, msg, field)
		}
	}
}
