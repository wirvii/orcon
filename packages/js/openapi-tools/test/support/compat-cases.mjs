// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Compatibility pairs (ADR-0054 section 3, VER-2/VER-3): each case is a small contract
 * `base` and a `head` made from it by one change, with the verdict `make api-compat` must
 * give. `testdata/compat/<case>/{base,head,expect}.yaml` are generated from this table by
 * `node test/support/write-compat-cases.mjs` and committed.
 *
 * expect:   `fail` (a breaking change) or `pass`;
 * detector: who reports a `fail`: `oasdiff` (with the severity file and events converted to
 *           responses) or `extra` (scripts/compat-extra.mjs, for `x-orcon-*` extensions);
 * check:    the oasdiff check id that the pair exercises when it is listed in
 *           oasdiff-levels.txt (every id of that file needs a pair).
 * `row` names the normative row of VER-2/VER-3 the pair stands for.
 */

import { stringify } from 'yaml';

const UUID = '0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a60';

export function baseDocument() {
  return {
    openapi: '3.1.0',
    info: { title: 'Compatibility fixture', version: '1.0.0' },
    paths: {
      '/flows': {
        get: {
          operationId: 'listFlows',
          summary: 'List flows',
          'x-orcon-stability': 'stable',
          'x-orcon-permission': 'flow:read',
          parameters: [
            { name: 'limit', in: 'query', required: false, schema: { type: 'integer', minimum: 1, maximum: 200, default: 50 } },
            { name: 'pageToken', in: 'query', required: false, schema: { type: 'string', maxLength: 1024 } },
            { name: 'sort', in: 'query', required: false, 'x-orcon-sort-default': '-updatedAt', schema: { type: 'string', enum: ['name', '-name', 'updatedAt', '-updatedAt'] } },
          ],
          responses: {
            200: { description: 'A page of flows.', content: { 'application/json': { schema: { $ref: '#/components/schemas/FlowList' } } } },
            401: { description: 'Not authenticated.' },
          },
        },
        post: {
          operationId: 'createFlow',
          summary: 'Create a flow',
          'x-orcon-stability': 'stable',
          'x-orcon-permission': 'flow:create',
          'x-orcon-idempotency': 'optional',
          'x-orcon-problems': ['validation-failed', 'conflict'],
          requestBody: { required: true, content: { 'application/json': { schema: { $ref: '#/components/schemas/CreateFlowRequest' } } } },
          responses: {
            201: { description: 'The flow.', content: { 'application/json': { schema: { $ref: '#/components/schemas/Flow' } } } },
            401: { description: 'Not authenticated.' },
          },
        },
      },
      '/flows/{flowId}': {
        parameters: [{ name: 'flowId', in: 'path', required: true, schema: { type: 'string', format: 'uuid' } }],
        get: {
          operationId: 'getFlow',
          summary: 'Get a flow',
          'x-orcon-stability': 'stable',
          responses: {
            200: { description: 'The flow.', content: { 'application/json': { schema: { $ref: '#/components/schemas/Flow' } } } },
            404: { description: 'Not found.' },
          },
        },
      },
      '/probes': {
        get: {
          operationId: 'listProbes',
          summary: 'List probes',
          'x-orcon-stability': 'experimental',
          responses: {
            200: { description: 'Probes.', content: { 'application/json': { schema: { type: 'object', properties: { items: { type: 'array', items: { type: 'string' } } } } } } },
          },
        },
      },
    },
    webhooks: {
      'flow.published': {
        post: {
          operationId: 'flowPublishedEvent',
          summary: 'Receive a flow published event',
          'x-orcon-stability': 'stable',
          'x-orcon-event': { type: 'flow.published', cloudEventsType: 'dev.orcon.flow.published.v1', permission: 'flow:read', stability: 'stable', since: '1.0.0' },
          requestBody: { required: true, content: { 'application/json': { schema: { $ref: '#/components/schemas/FlowPublishedEvent' } } } },
          responses: { 204: { description: 'Accepted.' } },
        },
      },
    },
    components: {
      schemas: {
        Flow: {
          type: 'object',
          required: ['id', 'name', 'status'],
          properties: {
            id: { type: 'string', format: 'uuid', examples: [UUID] },
            name: { type: 'string' },
            status: { type: 'string', enum: ['ACTIVE', 'INACTIVE'] },
            updatedAt: { type: 'string', format: 'date-time' },
          },
        },
        FlowList: {
          type: 'object',
          required: ['items'],
          properties: { items: { type: 'array', items: { $ref: '#/components/schemas/Flow' } }, nextPageToken: { type: 'string' } },
        },
        CreateFlowRequest: {
          type: 'object',
          additionalProperties: false,
          required: ['name'],
          properties: {
            name: { type: 'string', maxLength: 200 },
            note: { type: 'string', maxLength: 500 },
            status: { type: 'string', enum: ['ACTIVE', 'INACTIVE'], default: 'ACTIVE' },
          },
        },
        ProblemCode: {
          type: 'string',
          enum: ['validation-failed', 'conflict', 'precondition-failed'],
          'x-orcon-enum-kind': 'identifier',
          'x-orcon-problem-catalog': {
            'validation-failed': { status: 400, title: 'Validation failed', when: 'A parameter or the body is invalid.', retryable: false },
            conflict: { status: 409, title: 'Conflict', when: 'The state does not allow the operation.', retryable: false },
            'precondition-failed': { status: 412, title: 'Precondition failed', when: 'If-Match does not match.', retryable: false },
          },
        },
        FlowPublishedEvent: {
          type: 'object',
          required: ['id', 'type', 'data'],
          properties: {
            id: { type: 'string', format: 'uuid' },
            type: { type: 'string' },
            data: {
              type: 'object',
              required: ['flowId', 'version'],
              properties: { flowId: { type: 'string', format: 'uuid' }, version: { type: 'integer' }, note: { type: 'string' } },
            },
          },
        },
      },
    },
  };
}

const flowSchema = (d) => d.components.schemas.Flow;
const create = (d) => d.components.schemas.CreateFlowRequest;
const event = (d) => d.components.schemas.FlowPublishedEvent;
const catalog = (d) => d.components.schemas.ProblemCode['x-orcon-problem-catalog'];

const fail = (row, detector, mutate, check) => ({ expect: 'fail', detector, row, mutate, ...(check ? { check } : {}) });
const pass = (row, mutate, check) => ({ expect: 'pass', row, mutate, ...(check ? { check } : {}) });

export const CASES = {
  // ---- breaking (VER-2) detected by oasdiff ---------------------------------------------
  'remove-operation': fail('removing an operation', 'oasdiff', (d) => void delete d.paths['/flows/{flowId}'].get),
  'remove-path': fail('removing a path', 'oasdiff', (d) => void delete d.paths['/flows/{flowId}']),
  'remove-required-response-field': fail('removing a required response field', 'oasdiff', (d) => {
    delete flowSchema(d).properties.name;
    flowSchema(d).required = ['id', 'status'];
  }),
  'remove-response-field': fail('removing a response field', 'oasdiff', (d) => void delete flowSchema(d).properties.updatedAt, 'response-optional-property-removed'),
  'remove-response-enum-value': fail('removing a response enum value', 'oasdiff', (d) => void (flowSchema(d).properties.status.enum = ['ACTIVE']), 'response-property-enum-value-removed'),
  'add-required-request-parameter': fail('adding a required request parameter', 'oasdiff', (d) => {
    d.paths['/flows'].get.parameters.push({ name: 'projectId', in: 'query', required: true, schema: { type: 'string', format: 'uuid' } });
  }),
  'add-required-request-field': fail('adding a required request field', 'oasdiff', (d) => {
    create(d).properties.projectId = { type: 'string', format: 'uuid' };
    create(d).required = ['name', 'projectId'];
  }),
  'optional-request-field-becomes-required': fail('making an optional request field required', 'oasdiff', (d) => void (create(d).required = ['name', 'note'])),
  'remove-request-field': fail('removing a request field', 'oasdiff', (d) => void delete create(d).properties.note, 'request-property-removed'),
  'remove-request-parameter': fail('removing a request parameter', 'oasdiff', (d) => {
    d.paths['/flows'].get.parameters = d.paths['/flows'].get.parameters.filter((p) => p.name !== 'pageToken');
  }, 'request-parameter-removed'),
  'change-field-type': fail('changing a field type', 'oasdiff', (d) => void (flowSchema(d).properties.name = { type: 'integer' })),
  'change-field-format': fail('changing a field format', 'oasdiff', (d) => void (flowSchema(d).properties.id = { type: 'string', format: 'date-time' })),
  'narrow-request-enum': fail('narrowing accepted values (enum)', 'oasdiff', (d) => void (create(d).properties.status.enum = ['ACTIVE'])),
  'narrow-request-length': fail('narrowing accepted values (maxLength)', 'oasdiff', (d) => void (create(d).properties.name.maxLength = 100)),
  'stricter-pagination-limit': fail('stricter pagination (smaller maximum limit)', 'oasdiff', (d) => void (d.paths['/flows'].get.parameters[0].schema.maximum = 100)),
  'change-default': fail('changing an observable default (page size)', 'oasdiff', (d) => void (d.paths['/flows'].get.parameters[0].schema.default = 25), 'request-parameter-default-value-changed'),
  'change-request-property-default': fail('changing an observable default (request field)', 'oasdiff', (d) => void (create(d).properties.status.default = 'INACTIVE'), 'request-property-default-value-changed'),
  'remove-response-status': fail('removing a success response', 'oasdiff', (d) => {
    delete d.paths['/flows'].post.responses['201'];
    d.paths['/flows'].post.responses['200'] = { description: 'The flow.', content: { 'application/json': { schema: { $ref: '#/components/schemas/Flow' } } } };
  }),
  // ---- breaking (VER-2) detected by scripts/compat-extra.mjs ----------------------------
  'permission-stricter': fail('a stricter permission', 'extra', (d) => void (d.paths['/flows'].get['x-orcon-permission'] = 'flow:update')),
  'permission-all-of-grows': fail('a stricter permission (one more required)', 'extra', (d) => void (d.paths['/flows'].get['x-orcon-permission'] = { allOf: ['flow:read', 'execution:read'] })),
  'idempotency-becomes-required': fail('stricter idempotency', 'extra', (d) => void (d.paths['/flows'].post['x-orcon-idempotency'] = 'required')),
  'sort-default-changes': fail('changing the default sort', 'extra', (d) => void (d.paths['/flows'].get.parameters.find((p) => p.name === 'sort')['x-orcon-sort-default'] = 'name')),
  'problem-code-status-changes': fail('changing the status of a problem code', 'extra', (d) => void (catalog(d).conflict.status = 422)),
  'problem-code-removed': fail('removing a problem code from the catalog', 'extra', (d) => void delete catalog(d)['precondition-failed']),
  'problem-code-replaced': fail('changing the error code for the same condition', 'extra', (d) => {
    catalog(d)['already-exists'] = { status: 409, title: 'Already exists', when: 'It exists.', retryable: false };
    d.paths['/flows'].post['x-orcon-problems'] = ['validation-failed', 'already-exists'];
  }),
  'event-type-version-renamed': fail('renaming an event type (cloudEventsType)', 'extra', (d) => void (d.webhooks['flow.published'].post['x-orcon-event'].cloudEventsType = 'dev.orcon.flow.published.v2')),
  'event-permission-stricter': fail('a stricter permission to receive an event', 'extra', (d) => void (d.webhooks['flow.published'].post['x-orcon-event'].permission = 'flow:update')),
  // ---- events (webhooks), compared as responses -----------------------------------------
  'webhook-remove-event': fail('webhooks: removing an event type', 'oasdiff', (d) => void delete d.webhooks['flow.published']),
  'webhook-remove-required-field': fail('webhooks: removing a required event payload field', 'oasdiff', (d) => {
    delete event(d).properties.data.properties.version;
    event(d).properties.data.required = ['flowId'];
  }),
  'webhook-remove-optional-field': fail('webhooks: removing an optional event payload field', 'oasdiff', (d) => void delete event(d).properties.data.properties.note, 'response-optional-property-removed'),
  'webhook-change-field-type': fail('webhooks: changing the type of an event payload field', 'oasdiff', (d) => void (event(d).properties.data.properties.version = { type: 'string' })),
  // ---- non-breaking (VER-3) -------------------------------------------------------------
  'add-optional-request-field': pass('new optional request field', (d) => void (create(d).properties.description = { type: 'string', maxLength: 1000 })),
  'add-optional-request-parameter': pass('new optional request parameter', (d) => {
    d.paths['/flows'].get.parameters.push({ name: 'q', in: 'query', required: false, schema: { type: 'string', maxLength: 200 } });
  }),
  'add-response-field': pass('new response field', (d) => void (flowSchema(d).properties.description = { type: 'string' })),
  'add-response-enum-value': pass('new response enum value (clients ignore unknown values)', (d) => void (flowSchema(d).properties.status.enum = ['ACTIVE', 'INACTIVE', 'ARCHIVED']), 'response-property-enum-value-added'),
  'add-operation': pass('new operation', (d) => {
    d.paths['/flows/{flowId}'].delete = { operationId: 'deleteFlow', summary: 'Delete a flow', 'x-orcon-stability': 'stable', responses: { 204: { description: 'Deleted.' } } };
  }),
  'add-request-enum-value': pass('new request enum value', (d) => void (create(d).properties.status.enum = ['ACTIVE', 'INACTIVE', 'ARCHIVED'])),
  'add-error-response': pass('new error code for a new condition', (d) => {
    d.paths['/flows/{flowId}'].get.responses['409'] = { description: 'Conflict.' };
    catalog(d)['new-condition'] = { status: 409, title: 'New condition', when: 'A new case.', retryable: false };
    d.paths['/flows/{flowId}'].get['x-orcon-problems'] = ['new-condition'];
  }),
  'widen-request-length': pass('widening accepted values', (d) => void (create(d).properties.name.maxLength = 300)),
  'permission-looser': pass('a looser permission', (d) => void (d.paths['/flows'].get['x-orcon-permission'] = 'authenticated')),
  'experimental-change-ignored': pass('experimental operations are outside the guarantee (VER-1)', (d) => void delete d.paths['/probes']),
  'webhook-add-optional-field': pass('webhooks: new optional event payload field', (d) => void (event(d).properties.data.properties.projectId = { type: 'string', format: 'uuid' })),
  'webhook-add-required-field': pass('webhooks: new required event payload field (harmless for receivers)', (d) => {
    event(d).properties.data.properties.projectId = { type: 'string', format: 'uuid' };
    event(d).properties.data.required = ['flowId', 'version', 'projectId'];
  }),
};

/** The text of every case: Map(case -> { base, head, expect }). */
export function buildCompatCases() {
  const files = new Map();
  for (const [name, spec] of Object.entries(CASES)) {
    const head = baseDocument();
    spec.mutate(head);
    const lines = [`expect: ${spec.expect}`];
    if (spec.detector) lines.push(`detector: ${spec.detector}`);
    if (spec.check) lines.push(`check: ${spec.check}`);
    files.set(name, {
      base: stringify(baseDocument(), { lineWidth: 0 }),
      head: stringify(head, { lineWidth: 0 }),
      expect: `${lines.join('\n')}\n`,
    });
  }
  return files;
}
