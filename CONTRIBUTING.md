<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Contributing

Orcon welcomes issues, design discussions and pull requests. All source contributions are intended for Apache-2.0 distribution. Please include tests for behavior changes and document execution semantics before implementing durable engine behavior.

## Go code

Read [docs/engineering/go.md](docs/engineering/go.md) before changing Go code. The complete Go rulebook lives in `.agents/skills/orcon-go/references/` and is packaged as the `orcon-go` mneme skill. Run `make hooks` once after cloning and `make check` before opening a pull request; CI runs the same command.

## Web application code

Read [docs/engineering/web.md](docs/engineering/web.md) before changing code in `apps/web` or `packages/js`. The complete web application rulebook lives in `.agents/skills/orcon-web/references/` and is packaged as the `orcon-web` mneme skill.
