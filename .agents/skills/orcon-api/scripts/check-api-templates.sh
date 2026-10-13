#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Lints the skill's contract templates with the project lint: composes
# assets/templates/operation.yaml, list-operation.yaml, event.yaml and
# problem-code.yaml with the minimal shared components below into one temporary
# OpenAPI document, then runs `redocly lint` with the orcon plugin of
# packages/js/openapi-tools and its testdata/permissions.yaml. Expects 0 problems.
# The temporary files are removed on exit.
#
# Requires an Orcon checkout with packages/js/openapi-tools installed
# (pnpm install) and Node 22.
#
# Usage: sh .agents/skills/orcon-api/scripts/check-api-templates.sh

set -eu

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
skill_dir=$(CDPATH='' cd -- "$script_dir/.." && pwd)
repo_root=$(git -C "$script_dir" rev-parse --show-toplevel 2>/dev/null) || {
  echo "check-api-templates.sh: must run from inside an Orcon repository checkout" >&2
  exit 1
}
tools="$repo_root/packages/js/openapi-tools"
templates="$skill_dir/assets/templates"

for f in "$tools/redocly.yaml" "$tools/testdata/permissions.yaml"; do
  if [ ! -f "$f" ]; then
    echo "check-api-templates.sh: $f not found; the lint package packages/js/openapi-tools is required" >&2
    exit 1
  fi
done
if [ ! -d "$tools/node_modules" ]; then
  echo "check-api-templates.sh: run 'pnpm install' first (packages/js/openapi-tools has no node_modules)" >&2
  exit 1
fi

tmp=$(mktemp -d "${TMPDIR:-/tmp}/orcon-api-templates.XXXXXX")
trap 'rm -rf "$tmp"' EXIT INT TERM

# Minimal shared components the templates reference.
cat >"$tmp/base.yaml" <<'EOF'
openapi: 3.1.0
info:
  title: Orcon API templates
  version: 0.0.0
  description: Temporary document that composes the orcon-api skill templates for linting.
  license:
    name: Apache-2.0
    identifier: Apache-2.0
servers:
  - url: /api/v1
security:
  - bearer: []
tags:
  - name: flows
    description: Flows and their versions.
paths: {}
webhooks: {}
components:
  securitySchemes:
    bearer:
      type: http
      scheme: bearer
      description: An API key, service account token or session token.
  parameters:
    FlowId:
      name: flowId
      in: path
      required: true
      description: Id of the flow.
      schema:
        type: string
        format: uuid
        examples: [0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a60]
    ProjectId:
      name: projectId
      in: path
      required: true
      description: Id of the project.
      schema:
        type: string
        format: uuid
        examples: [0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a70]
    IfMatch:
      name: If-Match
      in: header
      required: true
      description: ETag of the representation the change is based on.
      schema:
        type: string
        maxLength: 255
        examples: ['"7"']
    IdempotencyKey:
      name: Idempotency-Key
      in: header
      required: false
      description: Client key that makes a retried request return the first response.
      schema:
        type: string
        minLength: 1
        maxLength: 255
        pattern: '^[\x21-\x7E]+$'
        examples: [publish-example-1]
    Limit:
      name: limit
      in: query
      required: false
      description: Maximum number of items in the page.
      schema:
        type: integer
        minimum: 1
        maximum: 200
        default: 50
        examples: [50]
    PageToken:
      name: pageToken
      in: query
      required: false
      description: Opaque token from nextPageToken of the previous page.
      schema:
        type: string
        maxLength: 1024
        examples: [example-page-token-2]
  headers:
    ETag:
      description: Strong entity tag of the returned representation.
      schema:
        type: string
        examples: ['"8"']
    OrconRequestId:
      description: Id of the request, also present in logs and problem documents.
      schema:
        type: string
        format: uuid
        examples: [0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a62]
    RateLimit:
      description: Remaining quota of the rate class in the current window.
      schema:
        type: string
        examples: ['"write";r=118;t=42']
    RateLimitPolicy:
      description: Quota policy of the rate class.
      schema:
        type: string
        examples: ['"write";q=120;w=60']
    RetryAfter:
      description: Seconds to wait before retrying.
      schema:
        type: integer
        minimum: 0
        examples: [30]
  schemas:
    Problem:
      type: object
      description: A problem document (RFC 9457).
      required: [type, title, status, code, requestId]
      properties:
        type:
          type: string
          format: uri
          description: urn:orcon:problem:<code>.
          examples: ['urn:orcon:problem:not-found']
        title:
          type: string
          description: Short summary of the problem type.
          examples: [Not found]
        status:
          type: integer
          minimum: 400
          maximum: 599
          description: HTTP status code.
          examples: [404]
        detail:
          type: string
          description: Explanation of this occurrence.
          examples: [The flow does not exist.]
        instance:
          type: string
          description: Path of the request that failed.
          examples: [/api/v1/flows/0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a60]
        code:
          $ref: '#/components/schemas/ProblemCode'
        requestId:
          type: string
          format: uuid
          description: Id of the request.
          examples: [0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a62]
        errors:
          type: array
          description: Field errors.
          items:
            $ref: '#/components/schemas/ProblemFieldError'
    ProblemFieldError:
      type: object
      description: One error of one request field.
      required: [pointer, code, message]
      properties:
        pointer:
          type: string
          description: JSON Pointer into the request.
          examples: [/note]
        code:
          type: string
          description: Stable code of the field error.
          examples: [too-long]
        message:
          type: string
          description: Explanation for a person.
          examples: [The note is longer than 500 characters.]
    ProblemCode:
      type: string
      description: Stable code of a problem; the type URI is urn:orcon:problem:<code>.
      x-orcon-enum-kind: identifier
      enum: [validation-failed, invalid-page-token, experimental-not-enabled, unauthenticated, forbidden, step-up-required, personal-keys-disabled, not-found, conflict, idempotency-in-progress, secret-already-issued, attention-already-resolved, deprecated-removed, precondition-failed, payload-too-large, idempotency-key-reused, precondition-required, rate-limited, internal-error, service-unavailable]
      x-orcon-problem-catalog:
        validation-failed: {status: 400, title: Validation failed, when: A parameter or the body is invalid., retryable: false}
        invalid-page-token: {status: 400, title: Invalid page token, when: The page token is unknown or was issued for other parameters., retryable: false}
        experimental-not-enabled: {status: 400, title: Experimental feature not enabled, when: The operation is experimental and Orcon-Experimental does not name it., retryable: false}
        unauthenticated: {status: 401, title: Unauthenticated, when: No valid credential was sent., retryable: false}
        forbidden: {status: 403, title: Forbidden, when: The caller lacks the required permission., retryable: false}
        step-up-required: {status: 403, title: Step-up required, when: The operation needs a recent second factor., retryable: false}
        personal-keys-disabled: {status: 403, title: Personal keys disabled, when: The organization does not allow personal API keys., retryable: false}
        not-found: {status: 404, title: Not found, when: The resource does not exist or is not visible to the caller., retryable: false}
        conflict: {status: 409, title: Conflict, when: The resource state does not allow the operation., retryable: false}
        idempotency-in-progress: {status: 409, title: Idempotent request in progress, when: A request with the same key is still running., retryable: true}
        secret-already-issued: {status: 409, title: Secret already issued, when: A replay asked again for a secret that is shown only once., retryable: false}
        attention-already-resolved: {status: 409, title: Attention already resolved, when: The human-attention item was already handled., retryable: false}
        deprecated-removed: {status: 410, title: Removed, when: The operation was removed after its sunset date., retryable: false}
        precondition-failed: {status: 412, title: Precondition failed, when: If-Match does not match the current ETag., retryable: false}
        payload-too-large: {status: 413, title: Payload too large, when: The body exceeds the size limit., retryable: false}
        idempotency-key-reused: {status: 422, title: Idempotency key reused, when: The key was used with a different request., retryable: false}
        precondition-required: {status: 428, title: Precondition required, when: If-Match is missing., retryable: false}
        rate-limited: {status: 429, title: Rate limited, when: The rate class quota is exhausted., retryable: true}
        internal-error: {status: 500, title: Internal error, when: An unexpected server error happened., retryable: true}
        service-unavailable: {status: 503, title: Service unavailable, when: The server is starting, stopping or overloaded., retryable: true}
    CloudEvent:
      type: object
      description: CloudEvents 1.0 attributes in structured JSON form.
      required: [specversion, id, source, type, time]
      properties:
        specversion:
          type: string
          description: CloudEvents version.
          enum: ['1.0']
          x-orcon-enum-kind: identifier
        id:
          type: string
          format: uuid
          description: Id of the event; consumers deduplicate by it.
          examples: [0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a65]
        source:
          type: string
          description: Path of the resource the event is about.
          maxLength: 2048
          examples: [/api/v1/flows/0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a60]
        type:
          type: string
          description: Versioned event type, dev.orcon.<area>.<event>.v<N>.
          maxLength: 255
          examples: [dev.orcon.flow.published.v1]
        time:
          type: string
          format: date-time
          description: When the change happened, in UTC.
          examples: ['2026-10-12T15:04:05Z']
        datacontenttype:
          type: string
          description: Media type of data.
          enum: [application/json]
          x-orcon-enum-kind: identifier
    PublishFlowRequest:
      type: object
      description: Options for publishing a flow draft.
      additionalProperties: false
      properties:
        note:
          type: string
          maxLength: 500
          description: Release note stored with the version.
          examples: [Retry the CRM step on rate limits]
    FlowVersion:
      type: object
      description: An immutable published version of a flow.
      required: [flowId, version, publishedAt, publishedByPrincipalId]
      properties:
        flowId:
          type: string
          format: uuid
          description: Id of the flow.
          examples: [0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a60]
        version:
          type: integer
          minimum: 1
          description: Version number.
          examples: [8]
        note:
          type: string
          description: Release note given at publication.
          examples: [Retry the CRM step on rate limits]
        publishedAt:
          type: string
          format: date-time
          description: When the version was published, in UTC.
          x-orcon-volatile: true
          examples: ['2026-10-12T15:04:05Z']
        publishedByPrincipalId:
          type: string
          format: uuid
          description: Id of the user or service account that published the version.
          examples: [0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a61]
    Flow:
      type: object
      description: A flow of a project.
      required: [id, projectId, name, status, updatedAt]
      properties:
        id:
          type: string
          format: uuid
          readOnly: true
          description: Id of the flow.
          examples: [0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a60]
        projectId:
          type: string
          format: uuid
          description: Id of the project that owns the flow.
          examples: [0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a70]
        name:
          type: string
          description: Display name of the flow.
          examples: [Sync CRM contacts]
        status:
          type: string
          description: Activation status of the flow.
          enum: [ACTIVE, INACTIVE]
        updatedAt:
          type: string
          format: date-time
          description: When the flow last changed, in UTC.
          examples: ['2026-10-12T15:04:05Z']
    FlowList:
      type: object
      description: One page of flows.
      required: [items]
      properties:
        items:
          type: array
          description: The flows of this page.
          items:
            $ref: '#/components/schemas/Flow'
        nextPageToken:
          type: string
          description: Token for the next page; absent on the last page.
          examples: [example-page-token-2]
  responses:
    ValidationFailed:
      description: A parameter or the body is invalid.
      content:
        application/problem+json:
          schema:
            $ref: '#/components/schemas/Problem'
    Unauthenticated:
      description: No valid credential was sent.
      content:
        application/problem+json:
          schema:
            $ref: '#/components/schemas/Problem'
    Forbidden:
      description: The caller lacks the required permission.
      content:
        application/problem+json:
          schema:
            $ref: '#/components/schemas/Problem'
    NotFound:
      description: The resource does not exist or is not visible to the caller.
      content:
        application/problem+json:
          schema:
            $ref: '#/components/schemas/Problem'
    Conflict:
      description: The resource state does not allow the operation.
      content:
        application/problem+json:
          schema:
            $ref: '#/components/schemas/Problem'
    PayloadTooLarge:
      description: The body exceeds the size limit.
      content:
        application/problem+json:
          schema:
            $ref: '#/components/schemas/Problem'
    IdempotencyKeyReused:
      description: The idempotency key was used with a different request.
      content:
        application/problem+json:
          schema:
            $ref: '#/components/schemas/Problem'
    PreconditionRequired:
      description: If-Match is missing.
      content:
        application/problem+json:
          schema:
            $ref: '#/components/schemas/Problem'
    RateLimited:
      description: The rate class quota is exhausted.
      headers:
        Retry-After:
          $ref: '#/components/headers/RetryAfter'
      content:
        application/problem+json:
          schema:
            $ref: '#/components/schemas/Problem'
    InternalError:
      description: An unexpected server error happened.
      content:
        application/problem+json:
          schema:
            $ref: '#/components/schemas/Problem'
EOF

# Compose: path templates go under paths; templates with top-level sections
# (webhooks, components) are merged at the root. Maps merge recursively and
# lists are joined without duplicates.
cat >"$tmp/compose.mjs" <<'EOF'
import { readFileSync, writeFileSync } from 'node:fs';
import { createRequire } from 'node:module';

const [toolsDir, baseFile, outFile, ...templateFiles] = process.argv.slice(2);
const YAML = createRequire(`${toolsDir}/package.json`)('yaml');

const isMap = (v) => v !== null && typeof v === 'object' && !Array.isArray(v);
function merge(a, b) {
  if (isMap(a) && isMap(b)) {
    const out = { ...a };
    for (const [k, v] of Object.entries(b)) out[k] = k in out ? merge(out[k], v) : v;
    return out;
  }
  if (Array.isArray(a) && Array.isArray(b)) {
    const seen = new Set(a.map((x) => JSON.stringify(x)));
    return [...a, ...b.filter((x) => !seen.has(JSON.stringify(x)))];
  }
  return b;
}

let doc = YAML.parse(readFileSync(baseFile, 'utf8'));
for (const file of templateFiles) {
  const t = YAML.parse(readFileSync(file, 'utf8'));
  const isPaths = Object.keys(t).every((k) => k.startsWith('/'));
  doc = merge(doc, isPaths ? { paths: t } : t);
}
// The problem-code template adds a reusable response for its example code. Point the
// 409 of the publish operation at it so the response is exercised like a real one.
const publish = doc.paths?.['/flows/{flowId}/publish']?.post;
if (publish?.responses?.['409'] && doc.components?.responses?.ProjectManagedByGit) {
  publish.responses['409'] = { $ref: '#/components/responses/ProjectManagedByGit' };
}

// Drop components nothing reachable references (the shared base carries more than any one
// template needs), so the built-in no-unused-components rule only reports real mistakes.
const sections = ['schemas', 'responses', 'parameters', 'headers'];
const refPattern = /"#\/components\/(schemas|responses|parameters|headers)\/([^"]+)"/g;
const used = new Set();
const queue = [JSON.stringify({ ...doc, components: undefined })];
while (queue.length > 0) {
  for (const [, section, name] of queue.pop().matchAll(refPattern)) {
    const key = `${section}/${name}`;
    if (used.has(key) || !doc.components[section]?.[name]) continue;
    used.add(key);
    queue.push(JSON.stringify(doc.components[section][name]));
  }
}
for (const section of sections) {
  for (const name of Object.keys(doc.components[section] ?? {})) {
    if (!used.has(`${section}/${name}`)) delete doc.components[section][name];
  }
}

writeFileSync(outFile, YAML.stringify(doc));
EOF

node "$tmp/compose.mjs" "$tools" "$tmp/base.yaml" "$tmp/openapi.yaml" \
  "$templates/operation.yaml" \
  "$templates/list-operation.yaml" \
  "$templates/event.yaml" \
  "$templates/problem-code.yaml"

# The package configuration with absolute plugin paths and the test permissions.
awk -v tools="$tools" '
  /permissionsFile:/ { sub(/permissionsFile:.*/, "permissionsFile: " tools "/testdata/permissions.yaml") }
  { gsub(/\.\/plugin\//, tools "/plugin/"); print }
' "$tools/redocly.yaml" >"$tmp/redocly.yaml"

cd "$tools"
pnpm exec redocly lint --config "$tmp/redocly.yaml" --max-problems 1000 "$tmp/openapi.yaml"
echo "check-api-templates: ok"
