#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Proves that every rule detects what it is meant to detect: it plants 19
# violations (SPEC-008 Annex E.6; 18 and 19 are the orgscope analyzer of
# SPEC-004) in a scratch copy of the module and fails
# unless each one is reported by the expected tool. A rule that exists but
# detects nothing is the failure this guards against.
#
# Environment: GOLANGCI_LINT (default .bin/golangci-lint), BIN (default .bin;
# holds spdxcheck, provenance and orconvet). Build them with `make tools`.
set -eu

root=$(cd "$(dirname "$0")/.." && pwd)
BIN=${BIN:-$root/.bin}
GOLANGCI_LINT=${GOLANGCI_LINT:-$BIN/golangci-lint}

for tool in "$GOLANGCI_LINT" "$BIN/spdxcheck" "$BIN/provenance" "$BIN/orconvet"; do
  if [ ! -x "$tool" ]; then
    echo "lint-fixtures: $tool not found; run 'make tools'" >&2
    exit 1
  fi
done

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM

# Scratch module: the real configuration and tree, never the working copy, so
# a crash cannot leave planted violations behind.
cp "$root/go.mod" "$root/go.sum" "$root/.golangci.yml" "$tmp/"
cp -R "$root/cmd" "$root/internal" "$tmp/"
rm -rf "$tmp/internal/adapter/webui/dist/assets"

# Stubs for modules that must be importable by the fixtures but are not (and
# must never become) dependencies of Orcon.
mkdir -p "$tmp/stubs/pgx" "$tmp/stubs/testify/assert"
printf 'module github.com/jackc/pgx/v5\n\ngo 1.25.0\n' > "$tmp/stubs/pgx/go.mod"
printf '// Package pgx is a stub for the lint fixtures.\npackage pgx\n\n// Conn is a stub.\ntype Conn struct{}\n' > "$tmp/stubs/pgx/pgx.go"
printf 'module github.com/stretchr/testify\n\ngo 1.25.0\n' > "$tmp/stubs/testify/go.mod"
printf '// Package assert is a stub for the lint fixtures.\npackage assert\n\n// True is a stub.\nfunc True() {}\n' > "$tmp/stubs/testify/assert/assert.go"
(
  cd "$tmp"
  GOWORK=off go mod edit \
    -require=github.com/jackc/pgx/v5@v5.0.0 -replace=github.com/jackc/pgx/v5="$tmp/stubs/pgx" \
    -require=github.com/stretchr/testify@v1.0.0 -replace=github.com/stretchr/testify="$tmp/stubs/testify"
)

header='// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0
'

# fixture <dir> <file> <body...>: writes a Go file with the SPDX header, a
# package comment and the given body into the scratch module.
fixture() {
  dir=$tmp/$1
  mkdir -p "$dir"
  pkg=$(basename "$1")
  printf '%s\n// Package %s is a planted lint fixture.\npackage %s\n\n%s\n' "$header" "$pkg" "$pkg" "$3" > "$dir/$2"
}

# 1-4: an area importing what areas must not import.
fixture internal/zzfixture1 a.go 'import "github.com/wirvii/orcon/internal/adapter/httpapi"

// Handler is planted.
var Handler = httpapi.HealthHandler'
fixture internal/zzfixture2 a.go 'import "database/sql"

// ErrNone is planted.
var ErrNone = sql.ErrNoRows'
fixture internal/zzfixture3 a.go 'import "net/http"

// OK is planted.
const OK = http.StatusOK'
fixture internal/zzfixture4 a.go 'import "encoding/json"

// Marshal is planted.
var Marshal = json.Marshal'
# 5: pgx outside the postgres adapters.
fixture internal/adapter/zzfixture5 a.go 'import "github.com/jackc/pgx/v5"

// Conn is planted.
type Conn = pgx.Conn'
# 6: an area importing the web UI adapter.
fixture internal/zzfixture6 a.go 'import "github.com/wirvii/orcon/internal/adapter/webui"

// Assets is planted.
var Assets = webui.Assets'
# 7: a test importing testify.
fixture internal/zzfixture7 a_test.go 'import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPlanted(t *testing.T) { assert.True() }'
# 8: fmt.Println and log.Printf.
fixture internal/zzfixture8 a.go 'import (
	"fmt"
	"log"
)

// Say is planted.
func Say() {
	fmt.Println("hello")
	log.Printf("hello")
}'
# 9: //nolint without an explanation.
fixture internal/zzfixture9 a.go 'import "os"

// Remove is planted: the directive suppresses real findings but gives no reason.
func Remove() {
	os.Remove("x") //nolint:gosec,errcheck
}'
# 10: exported identifier without a doc comment.
fixture internal/zzfixture10 a.go 'func Undocumented() {}'
# 11: indentation with spaces.
fixture internal/zzfixture11 a.go '// Spaced is planted.
func Spaced() int {
        return 1
}'
# 12: slog message that is not a constant.
fixture internal/zzfixture12 a.go 'import "log/slog"

// Log is planted.
func Log(logger *slog.Logger, msg string) {
	logger.Info(msg)
}'
# 13: error compared with ==.
fixture internal/zzfixture13 a.go 'import "io"

// IsEOF is planted.
func IsEOF(err error) bool {
	return err == io.EOF
}'
# 14: http.ListenAndServe without timeouts.
fixture internal/adapter/zzfixture14 a.go 'import "net/http"

// Serve is planted.
func Serve() error {
	return http.ListenAndServe(":0", nil)
}'
# 15: a Go file without the SPDX header.
mkdir -p "$tmp/internal/zzfixture15"
printf '// Package zzfixture15 is a planted lint fixture.\npackage zzfixture15\n' > "$tmp/internal/zzfixture15/a.go"
# 16: a secret passed to slog.
fixture internal/zzfixture16 a.go 'import "log/slog"

// Login is planted.
func Login(logger *slog.Logger, password string) {
	logger.Info("login", "password", password)
}'
# 17: an AI authorship mark in a comment. The assistant name is assembled at
# run time so that this script itself carries no such mark.
ai=Cla
ai=${ai}ude
mkdir -p "$tmp/internal/zzfixture17"
printf '%s\n// Package zzfixture17 is a planted lint fixture.\n//\n// Co-Authored-By: %s <noreply@example.com>\npackage zzfixture17\n' "$header" "$ai" > "$tmp/internal/zzfixture17/a.go"
# 18: a SQL driver call outside the data-access layer (orgscope).
fixture internal/zzfixture18 a.go 'import "database/sql"

// Wipe is planted.
func Wipe(db *sql.DB) error {
	_, err := db.Exec("DELETE FROM flow")
	return err
}'
# 19: SQL text built by concatenation inside the data-access layer (orgscope).
fixture internal/adapter/sqlite/zzfixture19 a.go 'import (
	"context"
	"database/sql"
)

// Wipe is planted.
func Wipe(ctx context.Context, db *sql.DB, table string) error {
	_, err := db.ExecContext(ctx, "DELETE FROM "+table)
	return err
}'

# One golangci-lint run covers the lint-detected cases. golangci-lint exits 1
# when it finds issues, which is the expected outcome here.
lint_out=$tmp/lint.out
(
  cd "$tmp"
  GOWORK=off "$GOLANGCI_LINT" run --config .golangci.yml --output.text.colors=false \
    --output.text.path=stdout --show-stats=false ./internal/zzfixture... ./internal/adapter/zzfixture...
) > "$lint_out" 2>&1 || true

if [ -n "${LINT_FIXTURES_VERBOSE:-}" ]; then
  cat "$lint_out"
fi

failed=0
pass() { printf 'ok    %2s %s\n' "$1" "$2"; }
fail() {
  printf 'FAIL  %2s %s\n' "$1" "$2" >&2
  failed=$((failed + 1))
}

# expect_lint <n> <description> <path fragment> <linter> [message fragment]
expect_lint() {
  if grep -E "$3/[a-z_0-9]*\.go:[0-9]+:[0-9]+:.*${5:-}.*\(($4)\)" "$lint_out" > /dev/null 2>&1; then
    pass "$1" "$2"
  else
    fail "$1" "$2 (expected a $4 finding in $3)"
  fi
}

expect_lint 1 "area imports an adapter" internal/zzfixture1 depguard
expect_lint 2 "area imports database/sql" internal/zzfixture2 depguard
expect_lint 3 "area imports net/http" internal/zzfixture3 depguard
expect_lint 4 "area imports encoding/json" internal/zzfixture4 depguard
expect_lint 5 "pgx outside the postgres adapters" internal/adapter/zzfixture5 depguard
expect_lint 6 "area imports the web UI adapter" internal/zzfixture6 depguard
expect_lint 7 "test imports testify" internal/zzfixture7 depguard
expect_lint 8 "fmt.Println and log.Printf" internal/zzfixture8 forbidigo
expect_lint 9 "//nolint without a reason" internal/zzfixture9 nolintlint explanation
expect_lint 10 "exported identifier without doc comment" internal/zzfixture10 revive
expect_lint 11 "indentation with spaces" internal/zzfixture11 'gofmt|goimports'
expect_lint 12 "slog message that is not constant" internal/zzfixture12 sloglint
expect_lint 13 "error compared with ==" internal/zzfixture13 errorlint
expect_lint 14 "http.ListenAndServe without timeouts" internal/adapter/zzfixture14 gosec

# 15-19 are detected by the project tools, run from the scratch module.
if (cd "$tmp" && "$BIN/spdxcheck" internal/zzfixture15/a.go) 2>&1 | grep -q 'missing SPDX'; then
  pass 15 "Go file without SPDX header"
else
  fail 15 "Go file without SPDX header (spdxcheck reported nothing)"
fi
if (cd "$tmp" && GOWORK=off go vet -vettool="$BIN/orconvet" ./internal/zzfixture16/ 2>&1) | grep -q 'sensitive value password'; then
  pass 16 "secret passed to slog"
else
  fail 16 "secret passed to slog (orconvet nosecretlog reported nothing)"
fi
if (cd "$tmp" && "$BIN/provenance" files internal/zzfixture17/a.go) 2>&1 | grep -q 'class A'; then
  pass 17 "AI authorship mark in a comment"
else
  fail 17 "AI authorship mark in a comment (provenance reported nothing)"
fi
if (cd "$tmp" && GOWORK=off go vet -vettool="$BIN/orconvet" ./internal/zzfixture18/ 2>&1) | grep -q 'outside the data-access layer'; then
  pass 18 "SQL driver call outside the data-access layer"
else
  fail 18 "SQL driver call outside the data-access layer (orconvet orgscope reported nothing)"
fi
if (cd "$tmp" && GOWORK=off go vet -vettool="$BIN/orconvet" ./internal/adapter/sqlite/zzfixture19/ 2>&1) | grep -q 'SQL text must be a constant'; then
  pass 19 "SQL text that is not a constant"
else
  fail 19 "SQL text that is not a constant (orconvet orgscope reported nothing)"
fi

if [ "$failed" -ne 0 ]; then
  echo "lint-fixtures: $failed of 19 planted violations were not detected; lint output follows" >&2
  cat "$lint_out" >&2
  exit 1
fi
echo "lint-fixtures: all 19 planted violations detected"
