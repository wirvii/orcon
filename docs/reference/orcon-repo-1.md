<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Repository format `orcon/repo@1`

Status: **Accepted** | Source: ADR-0057 §2–§4 (accepted 2026-10-12 via SPEC-005)

Layout of a Git repository that holds Orcon flows as code. ADR-0057 is normative; if this page and the ADR disagree, the ADR wins. This is a file format, not an API representation; its values keep the form shown.

## Layout
```
orcon.yaml                            # format: orcon/repo@1; projects: [{ slug, path }]
projects/<project-slug>/
  project.yaml                        # name, description, tags, activation map { <flow-slug>: active|inactive }
  flows/<flow-slug>.flow.json         # orcon/flow@1 (deterministic pretty serialization)
  credentials/<credential-name>.yaml  # name, type, description; per environment, optional field values that are ONLY $secrets references; never literal secrets
  variables.yaml                      # non-secret project variables (defaults)
  environments/<env>.yaml             # per-environment non-secret overrides
```

Flow slugs are stored in the flow (`ext."orcon.git".slug`) so renaming a flow does not move its file unless requested. Pinned test data, sample events and execution data are never exported.

## Deterministic serialization
UTF-8, LF, 2-space indented JSON, keys in schema order (unknown `ext` keys sorted), arrays in document order, trailing newline. `write(read(file))` is byte-identical for files Orcon wrote. `contentHash` remains SHA-256 over JCS (ADR-0007 §6).

## Credential binding by name
Nodes reference credentials by name (`credentials: { <slot>: { name } }`). At dispatch the name resolves to the credential with that name owned by, or shared to, the execution's project (ADR-0041 §5, ADR-0043). Each environment's project holds its own values (entered there, or as `$secrets` references to that environment's external store). Applying or publishing a flow with missing or type-mismatched names fails validation with the list of missing bindings.
