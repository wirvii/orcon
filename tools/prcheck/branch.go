// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"regexp"
)

// releaseBranch is the branch release-please keeps for the release pull request.
const releaseBranch = "release-please--branches--main"

// maxBranchLength is the longest accepted branch name.
const maxBranchLength = 60

var branchName = regexp.MustCompile(`^(feat|fix|perf|refactor|docs|test|build|ci|chore|revert)/[a-z0-9]+(-[a-z0-9]+)*$`)

// CheckBranchName reports a problem when the branch is not named
// "<type>/<slug>" with a slug of lowercase words, at most 60 characters
// (orcon-contributing BR rules). Forks may use any name, so this only warns.
func CheckBranchName(name string) error {
	if name == releaseBranch {
		return nil
	}
	if len(name) > maxBranchLength {
		return fmt.Errorf("branch name %q is longer than %d characters", name, maxBranchLength)
	}
	if !branchName.MatchString(name) {
		return fmt.Errorf("branch name %q does not match <type>/<slug> with a type from the commit types and a lowercase hyphenated slug", name)
	}
	return nil
}
