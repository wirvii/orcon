<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Data and live updates

Status: Proposed — pending owner approval

This document defines how the web application reads, caches and changes server data, and how live updates over server-sent events reach the cache. Server state is held by TanStack Query on top of the generated SDK `@orcon/sdk` (ADR 0078, ADR 0054). Rule IDs (`DT-…`) are stable.

Precedence: accepted ADRs > accepted `orcon-go` rules > `orcon-ui` rules for anything visual > this rulebook > reviewer preference.

## The SDK

- **DT-1** Only `@orcon/sdk` performs HTTP or opens event streams; `fetch`, `XMLHttpRequest`, `EventSource`, `WebSocket` are forbidden elsewhere (SPEC-005).
- **DT-2** SDK calls live only in `features/<area>/api/**` and in `app/live-events/`, `app/errors/`. Screens and components never import the SDK.

## Queries

- **DT-3** Each feature defines a key factory and options once, in `api/queries.ts`:

  ```ts
  export const flowKeys = {
    all: (org: OrgId) => [org, 'flows'] as const,
    lists: (org: OrgId) => [...flowKeys.all(org), 'list'] as const,
    list: (org: OrgId, filters: FlowFilters) => [...flowKeys.lists(org), filters] as const,
    detail: (org: OrgId, id: FlowId) => [...flowKeys.all(org), 'detail', id] as const,
  };

  export const flowQueries = {
    detail: (org: OrgId, id: FlowId) =>
      queryOptions({ queryKey: flowKeys.detail(org, id), queryFn: ... }),
  };
  ```

  Components and loaders use these options; no inline keys.
- **DT-4** Every key starts with the active organization id; switching organization clears the cache.
- **DT-5** `queryFn` parses the SDK result with the generated Zod schema (`parseWith`); a parse failure is a contract error (`errors.md` ER-5).
- **DT-7** Client defaults (`app/query-client.ts`): `staleTime` 30 s, `gcTime` 5 min, `retry` only for network failures (at most 2; HTTP 429/503 are already retried by the SDK), `refetchOnWindowFocus` on.
- **DT-8** Every screen that renders a query uses `app/query-state/QueryState` so loading (skeleton), empty, error with retry, forbidden (403: permission message, no retry) and data look and behave the same.
- **DT-9** Server data is never copied into component state, context or a store; derive with `select`.
- **DT-10** Lists use `useInfiniteQuery` with `pageToken` and `limit` ≤ 200 (SPEC-005 R-CONV-01); never offsets.
- **DT-11** Route loaders prefetch with `queryClient.ensureQueryData(options)`; components read the same options.

## Mutations

- **DT-6** Mutations live in `api/mutations.ts` (`mutationOptions`), declare their invalidations through the key factory or write the returned resource with `setQueryData`, send `If-Match` for editable resources and handle `412` with the conflict flow (SPEC-005 R-CONV-05). Idempotency keys are added by the SDK.
- **DT-17** Optimistic updates only for reversible edits with rollback; never for running a flow, approvals, resolving UNCERTAIN operations or anything with an external effect: the server decides.

## Live updates

- **DT-12** Live updates: one organization stream per tab through `app/live-events` (`LiveEventsProvider`); the per-execution stream only while an execution view is mounted. Each feature exports from `api/live.ts` a map `eventType → handler(event, queryClient)` that either patches a cached resource with `setQueryData` (when the event carries enough) or invalidates the narrowest keys. Never invalidate everything except on `stream.reset`.
- **DT-13** Events are parsed with their Zod schema; unknown types are ignored and counted.
- **DT-14** Invalidations are coalesced per key over 250 ms; patches apply immediately.
- **DT-15** While the stream is disconnected the dispatcher exposes state `paused`; screens show the "live updates paused" banner from `orcon-ui`; on reconnect (the SDK resumes with `Last-Event-ID`) or on `stream.reset`, every key registered by live handlers is invalidated.

## Hidden data

- **DT-16** `dataHidden: true` renders the redacted value of `orcon-ui` ([redaction](../../orcon-ui/references/components.md#redaction)); missing data is never shown as empty.
