// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"regexp"
)

var specLine = regexp.MustCompile(`(?m)^Spec: (SPEC-\d{3,}|n/a)[ \t\r]*$`)

// CheckSpecLine reports a problem when the pull request body has no
// "Spec: SPEC-NNN" or "Spec: n/a" line (orcon-contributing SD rules).
func CheckSpecLine(body string) error {
	if specLine.MatchString(body) {
		return nil
	}
	return errors.New("the description has no line \"Spec: SPEC-NNN\" (or \"Spec: n/a\")")
}
