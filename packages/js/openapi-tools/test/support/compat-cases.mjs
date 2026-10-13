// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Compatibility pairs (ADR-0054 section 3, VER-2/VER-3): each case is a small contract
 * `base` and a `head` made from it by one change, with the verdict `make api-compat` must
 * give. `testdata/compat/<case>/{base,head,expect}.yaml` are generated from this
 * table by `node test/support/write-compat-cases.mjs` and committed.
 *
 * expect: `fail`  oasdiff reports a breaking change (exit status 1 with --fail-on ERR);
 *         `pass`  no breaking change;
 *         `gap`   VER-2 calls the change breaking but oasdiff does not report it: the selftest
 *                 accepts "no breaking change" and lists the row as a coverage gap.
 * expectWarn: the same verdict when oasdiff runs with --fail-on WARN (defaults to `expect`).
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
          parameters: [
            { name: 'limit', in: 'query', required: false, schema: { type: 'integer', minimum: 1, maximum: 200, default: 50 } },
            { name: 'pageToken', in: 'query', required: false, schema: { type: 'string', maxLength: 1024 } },
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
            status: { type: 'string', enum: ['ACTIVE', 'INACTIVE'] },
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

export const CASES = {
  // ---- breaking (VER-2) ----------------------------------------------------------------
  'remove-operation': { expect: 'fail', row: 'removing an operation', mutate: (d) => void delete d.paths['/flows/{flowId}'].get },
  'remove-path': { expect: 'fail', row: 'removing a path', mutate: (d) => void delete d.paths['/flows/{flowId}'] },
  'remove-response-field': {
    expect: 'gap',
    row: 'removing a response field',
    mutate(d) {
      delete flowSchema(d).properties.updatedAt;
    },
  },
  'remove-required-response-field': {
    expect: 'fail',
    row: 'removing a required response field',
    mutate(d) {
      delete flowSchema(d).properties.name;
      flowSchema(d).required = ['id', 'status'];
    },
  },
  'remove-response-enum-value': {
    expect: 'gap',
    row: 'removing a response enum value',
    mutate(d) {
      flowSchema(d).properties.status.enum = ['ACTIVE'];
    },
  },
  'add-required-request-parameter': {
    expect: 'fail',
    row: 'adding a required request parameter',
    mutate(d) {
      d.paths['/flows'].get.parameters.push({ name: 'projectId', in: 'query', required: true, schema: { type: 'string', format: 'uuid' } });
    },
  },
  'add-required-request-field': {
    expect: 'fail',
    row: 'adding a required request field',
    mutate(d) {
      create(d).properties.projectId = { type: 'string', format: 'uuid' };
      create(d).required = ['name', 'projectId'];
    },
  },
  'optional-request-field-becomes-required': {
    expect: 'fail',
    row: 'making an optional request field required',
    mutate(d) {
      create(d).required = ['name', 'note'];
    },
  },
  'remove-request-field': {
    expect: 'gap',
    expectWarn: 'fail',
    row: 'removing a request field',
    mutate(d) {
      delete create(d).properties.note;
    },
  },
  'change-field-type': {
    expect: 'fail',
    row: 'changing a field type',
    mutate(d) {
      flowSchema(d).properties.name = { type: 'integer' };
    },
  },
  'change-field-format': {
    expect: 'fail',
    row: 'changing a field format',
    mutate(d) {
      flowSchema(d).properties.id = { type: 'string', format: 'date-time' };
    },
  },
  'narrow-request-enum': {
    expect: 'fail',
    row: 'narrowing accepted values (enum)',
    mutate(d) {
      create(d).properties.status.enum = ['ACTIVE'];
    },
  },
  'narrow-request-length': {
    expect: 'fail',
    row: 'narrowing accepted values (maxLength)',
    mutate(d) {
      create(d).properties.name.maxLength = 100;
    },
  },
  'stricter-pagination-limit': {
    expect: 'fail',
    row: 'stricter pagination (smaller maximum limit)',
    mutate(d) {
      d.paths['/flows'].get.parameters[0].schema.maximum = 100;
    },
  },
  'change-default': {
    expect: 'gap',
    row: 'changing an observable default (page size)',
    mutate(d) {
      d.paths['/flows'].get.parameters[0].schema.default = 25;
    },
  },
  'remove-response-status': {
    expect: 'fail',
    row: 'removing a success response',
    mutate(d) {
      delete d.paths['/flows'].post.responses['201'];
      d.paths['/flows'].post.responses['200'] = { description: 'The flow.', content: { 'application/json': { schema: { $ref: '#/components/schemas/Flow' } } } };
    },
  },
  // ---- non-breaking (VER-3) ------------------------------------------------------------
  'add-optional-request-field': {
    expect: 'pass',
    row: 'new optional request field',
    mutate(d) {
      create(d).properties.description = { type: 'string', maxLength: 1000 };
    },
  },
  'add-optional-request-parameter': {
    expect: 'pass',
    row: 'new optional request parameter',
    mutate(d) {
      d.paths['/flows'].get.parameters.push({ name: 'q', in: 'query', required: false, schema: { type: 'string', maxLength: 200 } });
    },
  },
  'add-response-field': {
    expect: 'pass',
    row: 'new response field',
    mutate(d) {
      flowSchema(d).properties.description = { type: 'string' };
    },
  },
  'add-operation': {
    expect: 'pass',
    row: 'new operation',
    mutate(d) {
      d.paths['/flows/{flowId}'].delete = { operationId: 'deleteFlow', summary: 'Delete a flow', 'x-orcon-stability': 'stable', responses: { 204: { description: 'Deleted.' } } };
    },
  },
  'add-request-enum-value': {
    expect: 'pass',
    row: 'new request enum value',
    mutate(d) {
      create(d).properties.status.enum = ['ACTIVE', 'INACTIVE', 'ARCHIVED'];
    },
  },
  'add-error-response': {
    expect: 'pass',
    row: 'new error code for a new condition',
    mutate(d) {
      d.paths['/flows/{flowId}'].get.responses['409'] = { description: 'Conflict.' };
    },
  },
  'widen-request-length': {
    expect: 'pass',
    row: 'widening accepted values',
    mutate(d) {
      create(d).properties.name.maxLength = 300;
    },
  },
  'experimental-change-ignored': {
    expect: 'pass',
    row: 'experimental operations are outside the guarantee (VER-1)',
    mutate(d) {
      delete d.paths['/probes'];
    },
  },
  // ---- events (webhooks) ---------------------------------------------------------------
  // oasdiff compares a webhook like an operation whose request body is the event payload.
  // It reports a removed event (ERR) and a removed payload field (WARN), but it judges the
  // payload as a REQUEST: adding a required field to an event (harmless for receivers) is
  // reported as breaking. H-API5-1 therefore asks for an event verifier in SPEC-005 API5.
  'webhook-remove-event': {
    expect: 'fail',
    row: 'webhooks: removing an event type',
    mutate(d) {
      delete d.webhooks['flow.published'];
    },
  },
  'webhook-remove-field': {
    expect: 'gap',
    expectWarn: 'fail',
    row: 'webhooks: removing an event payload field',
    mutate(d) {
      delete event(d).properties.data.properties.version;
      event(d).properties.data.required = ['flowId'];
    },
  },
  'webhook-change-field-type': {
    expect: 'fail',
    row: 'webhooks: changing the type of an event payload field',
    mutate(d) {
      event(d).properties.data.properties.version = { type: 'string' };
    },
  },
  'webhook-add-optional-field': {
    expect: 'pass',
    row: 'webhooks: new optional event payload field',
    mutate(d) {
      event(d).properties.data.properties.projectId = { type: 'string', format: 'uuid' };
    },
  },
  'webhook-add-required-field': {
    expect: 'fail',
    expectWarn: 'fail',
    row: 'webhooks: new required event payload field (FALSE POSITIVE: harmless for receivers)',
    mutate(d) {
      event(d).properties.data.properties.projectId = { type: 'string', format: 'uuid' };
      event(d).properties.data.required = ['flowId', 'version', 'projectId'];
    },
  },
};

/** The text of every case: Map(case -> { base, head, expect }). */
export function buildCompatCases() {
  const files = new Map();
  for (const [name, spec] of Object.entries(CASES)) {
    const head = baseDocument();
    spec.mutate(head);
    files.set(name, {
      base: stringify(baseDocument(), { lineWidth: 0 }),
      head: stringify(head, { lineWidth: 0 }),
      expect: `err: ${spec.expect}\nwarn: ${spec.expectWarn ?? spec.expect}\n`,
    });
  }
  return files;
}
