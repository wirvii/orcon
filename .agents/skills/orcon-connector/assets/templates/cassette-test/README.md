<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Recorded tests (`orcon/cassette@1`)

A cassette is one recorded case of one operation: the requests the connector sent, the responses the service returned and the items the operation must produce. It lives in `tests/<resource>.<operation>.<case>.json` inside the connector folder and is replayed offline on every change (`references/testing.md`, TE-1 to TE-5).

## Fields

| Field | Meaning |
|---|---|
| `format` | always `orcon/cassette@1` |
| `operation` | the operation name, `<resource>.<verb>` |
| `case` | short kebab-case name of the case (`ok`, `two-pages`, `limit`, `not-found`) |
| `example` | `true` for the one success case per operation shown in the documentation (DC-3) |
| `parameters` | the resolved node parameters of the case |
| `items` | the input items |
| `credential` | the credential type and the fixture used to replay it; never a value |
| `interactions` | every request and response in order; credential-applied values appear as `[REDACTED:credential.<field>]` |
| `expect` | `items` the operation must produce, or `error { class, code }` for an error case |
| `recorded` | when and with which Orcon version it was recorded, and the scrubber version and rules digest |

Non-JSON bodies are stored as `{ "base64": "…", "mediaType": "…" }`.

## Recorded values are kept as they are

A cassette is an internal test artifact. The values the service returned are stored exactly as received, including their spelling (NM-13); never rewrite them to UPPER_SNAKE or any other form. The only changes to a recorded value are the scrubber's placeholders (`[REDACTED]`, `person<n>@example.com`, `Person <n>`, `+15550100<nn>`).

## Recording

```sh
orcon connector record internal/adapter/connectors/acme --operation contact.create --params params.json --case ok --example
```

or, from Go, `ORCON_RECORD=1 go test -run TestContactCreate ./internal/adapter/connectors/acme/...` with the credential in `ORCON_LIVE_ACME_API_KEY`. The scrubber runs before the file is written; if the detector still finds a secret or personal data, nothing is written and the command names the JSON path and the rule.

Never edit a cassette by hand except to shorten long arrays; re-run the detector afterwards.

## Files in this template

- `contact.create.ok.json` — a one-request success case marked as the example.
- `contact.getMany.two-pages.json` — a paginated case with two requests.
- `record_test.go` — a Go test that records with `ORCON_RECORD=1` and replays otherwise.
