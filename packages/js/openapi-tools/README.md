<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# @orcon/openapi-tools

The contract toolchain of the public API (ADR 0080, rulebook in `.agents/skills/orcon-api/references/`). The package is private and never shipped; the only thing that reaches the binary is the reference site it builds. The contract itself, `packages/openapi/v1/openapi.yaml`, is written by the API cuts of the application design, not by this package.

| What | Where |
|---|---|
| Lint configuration (Redocly built-in rules + the `orcon` plugin, all errors, none disabled) | `redocly.yaml` |
| The 31 `orcon/*` rules and the 31 action verbs | `plugin/orcon.js`, `plugin/rules/`, `plugin/verbs.js` |
| Bundle (YAML for the Go generator, JSON for Go tests) | `scripts/bundle.mjs` |
| Lint the real contract | `scripts/lint.mjs` (does nothing while the contract does not exist) |
| Reference site (Scalar, same origin, no CDN, at most 3 MiB compressed) | `scripts/decorate.mjs`, `scripts/build-reference.mjs` |
| Generated pages | `scripts/problem-codes.mjs`, `scripts/operation-index.mjs` (both take `--check`) |
| Compatibility and changelog (oasdiff) | `scripts/stability-filter.mjs`, `scripts/compat-selftest.mjs`, `scripts/changelog.mjs`, `scripts/changelog-deprecations.mjs`, `scripts/changelog-selftest.mjs` |
| Fixtures | `testdata/good/` (zero problems), `testdata/bad/` (one planted case per rule), `testdata/compat/`, `testdata/changelog/`, `testdata/permissions.yaml` |

## Commands

```sh
pnpm --filter @orcon/openapi-tools test          # all self-tests (about 40 s)
pnpm --filter @orcon/openapi-tools gen:bad       # regenerate testdata/bad from testdata/good
node packages/js/openapi-tools/test/support/write-compat-cases.mjs   # regenerate testdata/compat and testdata/changelog
sh .agents/skills/orcon-api/scripts/check-api-templates.sh           # the skill templates must lint with zero problems
```

The oasdiff tests need an `oasdiff` binary (`ORCON_OASDIFF`, `oasdiff` on the path, or `go tool oasdiff` from `tools/` once the make cut adds it); they are skipped when none is found.

## Changing a rule

1. Change the rulebook file and ADR 0080 first (a rule that is not in the rulebook is not allowed).
2. Change the rule in `plugin/rules/` and, for a verb, `plugin/verbs.js` and the PATH-5 table in the same pull request.
3. Add or adjust the planted case in `test/support/bad-cases.mjs`, run `gen:bad`, and run the tests. A planted case must fail with its own rule and no other `orcon/*` rule.

## Reference site and the content security policy

`build-reference.mjs` writes `index.html`, `reference-config.js`, `openapi.json`, `scalar.standalone.js` (copied unchanged from `node_modules`) and `LICENSE-scalar.txt`. The configuration is a file of its own because inline scripts are not allowed. Measured with the policy `default-src 'self'; connect-src 'self'`: the page loads but renders unstyled, because Scalar injects its styles at run time (inline styles are blocked). With `default-src 'self'; connect-src 'self'; style-src 'self' 'unsafe-inline'` the site renders and sends only same-origin requests, and "try it" sends no cookies. The one remaining violation report is a blocked `eval` that the script tolerates (the page works). `scripts/serve-reference.mjs` serves a built site with the headers of the binary for a manual check.
