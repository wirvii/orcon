<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Contributing

Orcon welcomes issues, design discussions and pull requests. All source contributions are intended for Apache-2.0 distribution. Please include tests for behavior changes and document execution semantics before implementing durable engine behavior.

## Go code

Read [docs/engineering/go.md](docs/engineering/go.md) before changing Go code. The complete Go rulebook lives in `.agents/skills/orcon-go/references/` and is packaged as the `orcon-go` mneme skill. Run `make hooks` once after cloning and `make check` before opening a pull request; CI runs the same command.

## Public API

Read [docs/engineering/api.md](docs/engineering/api.md) before changing the OpenAPI contract in `packages/openapi/`. The complete API design rulebook lives in `.agents/skills/orcon-api/references/` and is packaged as the `orcon-api` mneme skill; start from its templates in `.agents/skills/orcon-api/assets/templates/` and paste `.agents/skills/orcon-api/assets/pr-api-checklist.md` into the pull request.
