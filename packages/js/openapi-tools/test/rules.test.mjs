// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Edge cases of each rule beyond the 31 planted cases: every scenario starts from the
 * bundled good document, applies one change and expects the named orcon rule to report it
 * (message fragment included). The scenarios that must NOT report are listed as `clean`.
 */

import assert from 'node:assert/strict';
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { after, before, test } from 'node:test';

import { stringify } from 'yaml';

import { bundle } from '../scripts/bundle.mjs';
import { readDocument } from '../scripts/doc-io.mjs';
import { packageDir } from '../scripts/lib.mjs';
import { lint } from './support/lint.mjs';

let work;
let base;
let counter = 0;

before(() => {
  work = mkdtempSync(join(tmpdir(), 'orcon-rules-'));
  const out = join(work, 'good.yaml');
  bundle(join(packageDir, 'testdata', 'good', 'openapi.yaml'), out);
  base = readDocument(out);
});

after(() => rmSync(work, { recursive: true, force: true }));

/** Lints a mutated copy of the good document; returns [{ ruleId, message }] of orcon rules. */
function findings(mutate) {
  const doc = structuredClone(base);
  mutate(doc);
  const file = join(work, `case-${(counter += 1)}.yaml`);
  writeFileSync(file, stringify(doc, { lineWidth: 0 }));
  return lint(file).filter((p) => p.ruleId.startsWith('orcon/'));
}

const FLOW = '/flows/{flowId}';
const PUBLISH = '/flows/{flowId}/publish';
const FLOWS = '/projects/{projectId}/flows';
const SYNC = '/projects/{projectId}/sync';
const schemas = (d) => d.components.schemas;
const getFlow = (d) => d.paths[FLOW].get;
const field = (name, extra = {}) => ({ type: 'string', description: `The ${name}.`, examples: ['x'], ...extra });

/** [description, rule, message fragment, mutate] */
const REPORTS = [
  ['path with three parameters', 'path-shape', 'at most two', (d) => {
    d.paths['/projects/{projectId}/flows/{flowId}/versions/{version}/x'] = structuredClone(d.paths[FLOW]);
  }],
  ['path without x-orcon-resource', 'path-shape', 'no x-orcon-resource', (d) => void delete d.paths[FLOW]['x-orcon-resource']],
  ['segment not in kebab-case', 'path-shape', 'not kebab-case', (d) => {
    d.paths['/flowItems/{flowId}'] = d.paths[FLOW];
    delete d.paths[FLOW];
  }],
  ['singular collection', 'path-shape', 'not plural', (d) => {
    d.paths['/projects/{projectId}/flow'] = d.paths[FLOWS];
    delete d.paths[FLOWS];
  }],
  ['POST on a view', 'path-shape', 'GET only', (d) => {
    d.paths['/orgs/{orgId}/awaiting-human'].post = structuredClone(d.paths[FLOWS].post);
  }],
  ['verb path declared as a collection', 'path-shape', 'action verb', (d) => {
    d.paths[PUBLISH]['x-orcon-resource'] = 'collection';
  }],
  ['protocol tag outside the allowed protocol paths', 'path-shape', 'not an allowed protocol path', (d) => {
    getFlow(d).tags = ['flows', 'protocol'];
  }],
  ['GET on an action', 'action-verb', 'POST only', (d) => {
    d.paths[PUBLISH].get = structuredClone(getFlow(d));
  }],
  ['action whose last segment is a parameter', 'path-shape', 'not a valid action', (d) => {
    d.paths[FLOW]['x-orcon-resource'] = 'action';
  }],
  ['non-camel path parameter', 'path-param-name', 'must be <resource>Id', (d) => {
    d.paths['/flows/{flow_id}'] = structuredClone(d.paths[FLOW]);
  }],
  ['non-uuid path parameter', 'path-param-name', 'format: uuid', (d) => {
    d.components.parameters.FlowId.schema = { type: 'string', examples: ['abc'], maxLength: 10 };
  }],
  ['operationId not lowerCamel', 'operation-id', 'not lowerCamel', (d) => void (getFlow(d).operationId = 'GetFlow')],
  ['operationId without a verb', 'operation-id', 'must start with a verb', (d) => void (getFlow(d).operationId = 'flowById')],
  ['header without the Orcon prefix', 'header-names', 'does not use the Orcon- prefix', (d) => {
    getFlow(d).parameters = [{ name: 'Request-Id', in: 'header', description: 'Request id.', schema: { type: 'string', maxLength: 36, examples: ['1'] } }];
  }],
  ['array of non-uuid ids', 'id-format', 'array of uuid', (d) => {
    schemas(d).Flow.properties.memberIds = { type: 'array', description: 'Members.', items: { type: 'integer', examples: [1] } };
  }],
  ['non-uuid id query parameter', 'id-format', 'format: uuid', (d) => {
    d.paths[FLOWS].get.parameters.push({ name: 'ownerId', in: 'query', description: 'Owner.', schema: { type: 'integer', examples: [1] } });
  }],
  ['date-time on a field not named *At', 'time-format', 'not named *At', (d) => {
    schemas(d).Flow.properties.created = { type: 'string', format: 'date-time', description: 'Created.', examples: ['2026-10-12T15:04:05Z'] };
  }],
  ['*Date without format: date', 'time-format', 'format: date', (d) => {
    schemas(d).Flow.properties.startDate = field('start date');
  }],
  ['duration without format: duration', 'time-format', 'format: duration', (d) => {
    schemas(d).Flow.properties.timeoutDuration = field('timeout', { examples: ['PT5M'] });
  }],
  ['*Bytes that is not an integer', 'time-format', 'type: integer', (d) => {
    schemas(d).Flow.properties.sizeBytes = field('size');
  }],
  ['secret name without writeOnly', 'secret-fields', 'writeOnly', (d) => {
    schemas(d).Flow.properties.clientSecret = field('client secret', { examples: ['example-secret-1'] });
  }],
  ['secret example that is not a marker', 'secret-fields', 'marker', (d) => {
    schemas(d).Flow.properties.clientSecret = field('client secret', { writeOnly: true, examples: ['hunter2'] });
  }],
  ['secret value in a media example', 'secret-fields', 'marker', (d) => {
    getFlow(d).responses['200'].content['application/json'].examples.getExistingFlow.value.password = 'hunter2';
  }],
  ['request schema left open', 'named-schemas', 'additionalProperties: false', (d) => {
    delete schemas(d).CreateFlowRequest.additionalProperties;
  }],
  ['unbounded request string', 'named-schemas', 'maxLength', (d) => {
    delete schemas(d).CreateFlowRequest.properties.name.maxLength;
  }],
  ['unbounded request array', 'named-schemas', 'maxItems', (d) => {
    schemas(d).CreateFlowRequest.properties.tags = { type: 'array', description: 'Tags.', items: { type: 'string', maxLength: 10, examples: ['a'] } };
  }],
  ['array at the root of a response', 'response-shape', 'not a array', (d) => {
    schemas(d).FlowArray = { type: 'array', items: { $ref: '#/components/schemas/Flow' } };
    getFlow(d).responses['200'].content['application/json'].schema = { $ref: '#/components/schemas/FlowArray' };
  }],
  ['list with an extra member', 'response-shape', 'not allowed', (d) => {
    schemas(d).FlowList.properties.count = { type: 'integer', description: 'Count.', examples: [1] };
  }],
  ['list total without x-orcon-include-total', 'response-shape', 'x-orcon-include-total', (d) => {
    schemas(d).FlowList.properties.total = { type: 'integer', description: 'Total.', examples: [1] };
  }],
  ['list items that are not a named schema', 'response-shape', 'named schema', (d) => {
    schemas(d).FlowList.properties.items.items = { type: 'object', description: 'Inline.', properties: {} };
  }],
  ['boolean status at the root', 'response-shape', 'boolean "status"', (d) => {
    schemas(d).Flow.properties.status = { type: 'boolean', description: 'Ok.' };
  }],
  ['envelope member error', 'response-shape', '"error" is an envelope member', (d) => {
    schemas(d).Flow.properties.error = field('error');
  }],
  ['204 with a body', 'response-shape', '204 has no body', (d) => {
    d.paths[FLOW].delete.responses['204'].content = { 'application/json': { schema: { $ref: '#/components/schemas/Flow' } } };
  }],
  ['offset pagination', 'list-pagination', '"offset" is not allowed', (d) => {
    d.paths[FLOWS].get.parameters.push({ name: 'offset', in: 'query', description: 'Offset.', schema: { type: 'integer', examples: [0] } });
  }],
  ['limit with the wrong bounds', 'list-pagination', 'maximum 200', (d) => {
    d.components.parameters.Limit.schema.maximum = 500;
  }],
  ['includeTotal without the extension', 'list-pagination', 'x-orcon-include-total', (d) => {
    d.paths[FLOWS].get.parameters.push({ name: 'includeTotal', in: 'query', description: 'Total.', schema: { type: 'boolean' } });
  }],
  ['sort default outside the allowlist', 'sort-allowlist', 'not one of the allowed', (d) => {
    d.paths[FLOWS].get.parameters.find((p) => p.name === 'sort')['x-orcon-sort-default'] = 'size';
  }],
  ['error response with another schema', 'problem-responses', 'Problem', (d) => {
    d.components.responses.NotFound.content['application/problem+json'].schema = { $ref: '#/components/schemas/Flow' };
  }],
  ['error response with an extra media type', 'problem-responses', 'only application/problem+json', (d) => {
    d.components.responses.NotFound.content['application/json'] = { schema: { $ref: '#/components/schemas/Problem' } };
  }],
  ['problem code outside the catalog', 'problem-codes-declared', 'not in the catalog', (d) => {
    getFlow(d)['x-orcon-problems'].push('made-up');
  }],
  ['problem code without a response of its status', 'problem-codes-declared', 'no 409 response', (d) => {
    getFlow(d)['x-orcon-problems'].push('conflict');
  }],
  ['catalog code missing from the enum', 'problem-codes-declared', 'no catalog entry', (d) => {
    schemas(d).ProblemCode.enum.push('extra-code');
  }],
  ['catalog entry without retryable', 'problem-codes-declared', 'needs status', (d) => {
    delete schemas(d).ProblemCode['x-orcon-problem-catalog']['not-found'].retryable;
  }],
  ['x-orcon-problems missing', 'problem-codes-declared', 'declares no x-orcon-problems', (d) => void delete getFlow(d)['x-orcon-problems']],
  ['required idempotency with an optional header', 'idempotency', 'required header', (d) => {
    d.paths[PUBLISH].post['x-orcon-idempotency'] = 'required';
  }],
  ['long-running operation with optional idempotency', 'idempotency', 'must be required', (d) => {
    d.paths[SYNC].post['x-orcon-idempotency'] = 'optional';
  }],
  ['GET of an editable resource without ETag', 'concurrency', 'ETag', (d) => {
    delete getFlow(d).responses['200'].headers.ETag;
  }],
  ['PATCH without 428', 'concurrency', '428', (d) => {
    delete d.paths[FLOW].patch.responses['428'];
  }],
  ['stream with the read class', 'rate-class', 'stream class', (d) => {
    d.paths['/events/stream'].get['x-orcon-rate-class'] = 'read';
  }],
  ['mutation with the read class', 'rate-class', 'write or run class', (d) => {
    d.paths[PUBLISH].post['x-orcon-rate-class'] = 'read';
  }],
  ['429 without Retry-After', 'rate-class', 'Retry-After', (d) => {
    delete d.components.responses.RateLimited.headers;
  }],
  ['long-running without a 202', 'long-running', '202', (d) => {
    d.paths[SYNC].post.responses['200'] = d.paths[SYNC].post.responses['202'];
    delete d.paths[SYNC].post.responses['202'];
  }],
  ['long-running 202 that is not an Operation', 'long-running', 'Operation resource', (d) => {
    d.paths[SYNC].post.responses['202'].content['application/json'].schema = { $ref: '#/components/schemas/Flow' };
  }],
  ['experimental operation without the header', 'stability', 'Orcon-Experimental', (d) => {
    getFlow(d)['x-orcon-stability'] = 'experimental';
  }],
  ['sunset without deprecated', 'deprecation', 'not deprecated: true', (d) => {
    getFlow(d)['x-orcon-sunset'] = '2027-01-01';
  }],
  ['sunset that is not a date', 'deprecation', 'RFC 3339', (d) => {
    Object.assign(getFlow(d), { deprecated: true, 'x-orcon-sunset': 'soon', 'x-orcon-deprecation-guide': 'https://orcon.example/g' });
  }],
  ['deprecated field without a guide', 'deprecation', 'x-orcon-deprecation-guide', (d) => {
    Object.assign(schemas(d).Flow.properties.name, { deprecated: true, 'x-orcon-sunset': '2027-01-01' });
  }],
  ['anonymous on a normal operation', 'permission', 'protocol operations', (d) => void (getFlow(d)['x-orcon-permission'] = 'anonymous')],
  ['allOf with an unknown permission', 'permission', 'not in the permission catalog', (d) => {
    getFlow(d)['x-orcon-permission'] = { allOf: ['flow:read', 'flow:peek'] };
  }],
  ['permission of the wrong shape', 'permission', 'x-orcon-permission is a permission', (d) => void (getFlow(d)['x-orcon-permission'] = { oneOf: ['flow:read'] })],
  ['audit action in the wrong form', 'audit', 'lowercase', (d) => void (d.paths[PUBLISH].post['x-orcon-audit'] = 'FlowPublished')],
  ['duplicate CLI command', 'cli-declared', 'already used', (d) => void (getFlow(d)['x-orcon-cli'].command = 'flows list')],
  ['CLI command with four words', 'cli-declared', 'two or three words', (d) => void (getFlow(d)['x-orcon-cli'].command = 'flows get the one')],
  ['CLI exemption without a reason', 'cli-declared', 'category', (d) => void (getFlow(d)['x-orcon-cli'] = { none: { category: 'protocol' } })],
  ['CLI with both command and none', 'cli-declared', 'exactly one', (d) => void (getFlow(d)['x-orcon-cli'].none = { category: 'protocol', reason: 'r' })],
  ['duplicate MCP tool', 'mcp-declared', 'already used', (d) => void (getFlow(d)['x-orcon-mcp'].tool = 'search_flows')],
  ['MCP tool not in snake_case', 'mcp-declared', 'snake_case', (d) => void (getFlow(d)['x-orcon-mcp'].tool = 'GetFlow')],
  ['MCP level read on a POST', 'mcp-declared', 'changes state', (d) => void (d.paths[PUBLISH].post['x-orcon-mcp'].level = 'read')],
  ['MCP tool classified differently in V1', 'mcp-declared', 'classified destructive in V1', (d) => void (d.paths[PUBLISH].post['x-orcon-mcp'].level = 'write')],
  ['MCP DELETE that is not destructive', 'mcp-declared', 'DELETE is destructive', (d) => {
    d.paths[FLOW].delete['x-orcon-mcp'] = { level: 'write', tool: 'remove_flow' };
  }],
  ['MCP with an unknown key', 'mcp-declared', 'not allowed', (d) => void (getFlow(d)['x-orcon-mcp'].extra = true)],
  ['one-sentence description', 'doc-operation', 'at least two sentences', (d) => {
    getFlow(d).description = 'Returns one flow with the ETag of its current representation and nothing else at all.';
  }],
  ['hand-written generated section', 'doc-operation', 'generates', (d) => {
    getFlow(d).description += '\n\n**Required permission:** flow:read\n';
  }],
  ['summary over 80 characters', 'doc-operation', 'limit is 80', (d) => void (getFlow(d).summary = `Get ${'a very long flow name '.repeat(5)}`)],
  ['integer property without an example', 'doc-fields', 'without examples', (d) => {
    schemas(d).Flow.properties.revision = { type: 'integer', description: 'Revision.' };
  }],
  ['parameter without a description', 'doc-fields', 'has no description', (d) => {
    delete d.components.parameters.Limit.description;
  }],
  ['example naming a missing response example', 'doc-examples', 'does not define', (d) => {
    getFlow(d)['x-orcon-examples'][0].response.body = 'nope';
  }],
  ['example with a non kebab-case scenario', 'doc-examples', 'kebab-case', (d) => {
    getFlow(d)['x-orcon-examples'][0].scenario = 'Not Kebab';
  }],
  ['example for a status the operation lacks', 'doc-examples', 'no such response', (d) => {
    getFlow(d)['x-orcon-examples'][0].response.status = 201;
  }],
  ['stream example with an unknown trigger', 'doc-examples', 'not the name of an example', (d) => {
    d.paths['/events/stream'].get['x-orcon-examples'][0].stream.trigger = 'nope';
  }],
  ['examples without a 2xx case', 'doc-examples', 'at least one 2xx', (d) => {
    d.paths[FLOW].patch['x-orcon-examples'] = [{ name: 'staleRename', scenario: 'project-with-three-flows', request: {}, response: { status: 412 } }];
  }],
  ['event key different from its type', 'events', 'webhooks key', (d) => {
    d.webhooks['flow.activated'] = d.webhooks['flow.published'];
    delete d.webhooks['flow.published'];
  }],
  ['event without a payload example', 'events', 'has an example', (d) => {
    delete d.webhooks['flow.published'].post.requestBody.content['application/cloudevents+json'].examples;
  }],
  ['event type in the wrong form', 'events', 'lowercase snake', (d) => {
    d.webhooks['flow.published'].post['x-orcon-event'].type = 'FlowPublished';
  }],
  ['event without permission', 'events', 'permission', (d) => {
    delete d.webhooks['flow.published'].post['x-orcon-event'].permission;
  }],
  ['execution data without the protection mark', 'events', 'x-orcon-data: protected', (d) => {
    schemas(d).FlowPublishedData.properties.outputData = { type: 'object', description: 'Data.', properties: {} };
  }],
  ['prefixItems', 'codegen-subset', 'prefixItems', (d) => {
    schemas(d).Flow.properties.pair = { type: 'array', description: 'Pair.', prefixItems: [{ type: 'string' }] };
  }],
  ['type array with two types', 'codegen-subset', 'only as [T, "null"]', (d) => {
    schemas(d).Flow.properties.either = { type: ['string', 'integer'], description: 'Either.', examples: ['a'] };
  }],
  ['oneOf without a discriminator', 'codegen-subset', 'discriminator', (d) => {
    schemas(d).Flow.properties.either = { description: 'Either.', oneOf: [{ $ref: '#/components/schemas/Operation' }, { $ref: '#/components/schemas/FlowVersion' }] };
  }],
  ['not', 'codegen-subset', '"not"', (d) => {
    schemas(d).Flow.properties.other = { description: 'Other.', not: { type: 'string' } };
  }],
];

for (const [description, rule, fragment, mutate] of REPORTS) {
  test(`orcon/${rule} reports: ${description}`, () => {
    const found = findings(mutate).filter((f) => f.ruleId === `orcon/${rule}`);
    assert.ok(found.length > 0, `orcon/${rule} reported nothing`);
    assert.ok(found.some((f) => f.message.includes(fragment)), `no message of orcon/${rule} contains "${fragment}":\n${found.map((f) => f.message).join('\n')}`);
  });
}

/** Changes that are fine and must stay clean. */
const CLEAN = [
  ['a protocol-tagged operation on an allowed path (OIDC callback)', (d) => {
    d.paths['/oidc/callback'] = structuredClone(d.paths['/oauth/token']);
    d.paths['/oidc/callback'].post.operationId = 'oidcCallback';
  }],
  ['an identifier enum', (d) => void (schemas(d).Flow.properties.kind = { type: 'string', description: 'Kind.', enum: ['a-b', 'c'], 'x-orcon-enum-kind': 'identifier' })],
  ['a provider id with x-orcon-external-id', (d) => void (schemas(d).Flow.properties.providerId = { type: 'string', description: 'Id at the provider.', maxLength: 64, examples: ['abc'], 'x-orcon-external-id': true })],
  ['a writeOnly secret with a marker', (d) => void (schemas(d).Flow.properties.apiKey = { type: 'string', format: 'password', writeOnly: true, description: 'Key.', examples: ['example-secret-1'] })],
  ['a nullable string', (d) => void (schemas(d).Flow.properties.note = { type: ['string', 'null'], description: 'Note.', examples: ['n'] })],
  ['a plural exception', (d) => {
    d.paths['/projects/{projectId}/flow-state'] = structuredClone(d.paths[FLOWS]);
    d.paths['/projects/{projectId}/flow-state']['x-orcon-plural-exception'] = 'The state is one invariant noun.';
    d.paths['/projects/{projectId}/flow-state'].get.operationId = 'listFlowState';
    d.paths['/projects/{projectId}/flow-state'].get['x-orcon-cli'].command = 'flow-state list';
    d.paths['/projects/{projectId}/flow-state'].get['x-orcon-mcp'] = { level: 'none' };
    d.paths['/projects/{projectId}/flow-state'].post.operationId = 'createFlowState';
    d.paths['/projects/{projectId}/flow-state'].post['x-orcon-cli'].command = 'flow-state create';
    d.paths['/projects/{projectId}/flow-state'].post['x-orcon-mcp'] = { level: 'none' };
  }],
  ['x-orcon-* values that are not UPPER_SNAKE', (d) => void (getFlow(d)['x-orcon-note'] = { kind: 'lower-case' })],
  ['an experimental operation with its header and problem', (d) => {
    getFlow(d)['x-orcon-stability'] = 'experimental';
    getFlow(d).parameters = [{ name: 'Orcon-Experimental', in: 'header', required: true, description: 'Feature flag.', schema: { type: 'string', maxLength: 64, examples: ['flows-get'] } }];
    getFlow(d)['x-orcon-problems'].push('experimental-not-enabled');
    delete getFlow(d)['x-orcon-examples'];
  }],
];

for (const [description, mutate] of CLEAN) {
  test(`the orcon rules accept ${description}`, () => {
    assert.deepEqual(findings(mutate).map((f) => `${f.ruleId}: ${f.message}`), []);
  });
}

test('the plugin does not crash on thin or odd documents', () => {
  const odd = [
    { openapi: '3.1.0', info: { title: 't', version: '1' }, paths: {} },
    { openapi: '3.1.0', info: { title: 't', version: '1' }, paths: { '/x': { get: {} } } },
    { openapi: '3.1.0', info: { title: 't', version: '1' }, paths: { '/x': { get: { responses: { 200: { description: 'ok' } } } } }, webhooks: { e: { post: {} } } },
    { openapi: '3.1.0', info: { title: 't', version: '1' }, paths: { '/x': { get: { parameters: [{ $ref: '#/components/parameters/Missing' }], responses: { 200: { $ref: '#/components/responses/Missing' } } } } } },
  ];
  for (const doc of odd) {
    const file = join(work, `odd-${(counter += 1)}.yaml`);
    writeFileSync(file, stringify(doc));
    const problems = lint(file);
    assert.ok(Array.isArray(problems));
    assert.ok(!problems.some((p) => /TypeError|Cannot read|is not a function/.test(p.message)), JSON.stringify(problems));
  }
});
