<!--
Title: a Conventional Commit; it becomes the single commit on main.
  <type>(<scope>): <summary in lowercase, imperative, no final period>
  Scopes: engine, trigger, connector, api, cli, mcp, web, ui, db, ai, security, notify, docs, ci, deps, tooling
  Breaking change: add ! before the colon, for example feat(api)!: remove the v0 flow endpoint
Rules: CONTRIBUTING.md and .agents/skills/orcon-contributing/references/pull-requests.md
-->

## What and why

<!-- What changes and why it is needed. Self-contained: readers must not need any other tool to understand it. -->

Spec: <!-- SPEC-NNN for maintainer work tracked in mneme, otherwise n/a -->

## How it was tested

<!-- Commands you ran, their results, and known limitations. -->

## Screenshots

<!-- UI changes: before and after, light and dark theme. Otherwise write "Not a UI change". -->

## Size justification

<!-- Only when more than 400 lines change, not counting generated code, translation catalogs, screenshots and lockfiles: why it cannot be split. Otherwise write "None". -->

## Breaking changes

<!-- Who is affected and how to migrate, or "None". -->

## Checklists

<!-- Tick every area this pull request touches and paste that skill's checklist below this list.
     sh .agents/skills/orcon-contributing/scripts/compose-pr-body.sh does it for you. -->

- [ ] orcon-ui checklist pasted below (design system, components, stories, canvas)
- [ ] orcon-web checklist pasted below (editor application code)
- [ ] orcon-go checklist pasted below (Go code, modules, generated Go)
- [ ] orcon-db checklist pasted below (schema, migrations, queries)
- [ ] orcon-api checklist pasted below (API contract, events, CLI, MCP)
- [ ] orcon-connector checklist pasted below (connectors)

### Every pull request

- [ ] The title is a Conventional Commit with one scope from the list.
- [ ] Tests cover the change, and "How it was tested" lists the commands and results.
- [ ] Documentation and translations are updated where behavior or text changed.
- [ ] User-visible behavior changed: `docs/user/en` and `docs/user/es` updated in this pull request
- [ ] No secrets, credentials or personal data in code, fixtures, logs or screenshots.
- [ ] Code comments, commit messages and this description contain no AI authorship trailers or AI tool attributions.
- [ ] Any new dependency has an allowed license and a reason in "What and why".
