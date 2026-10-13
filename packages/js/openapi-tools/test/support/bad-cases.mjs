// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * The planted cases: one small document per `orcon/*` rule, each derived from the good
 * document by a single mutation. `testdata/bad/<case>.yaml` are generated from this table
 * (`node test/support/write-bad-cases.mjs`) and committed; `bad.test.mjs` checks that the
 * committed files equal the generated ones and that each one fails with exactly its rule.
 */

import { mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

import { parse, stringify } from 'yaml';

import { bundle } from '../../scripts/bundle.mjs';
import { packageDir } from '../../scripts/lib.mjs';

const METHODS = ['get', 'put', 'post', 'delete', 'options', 'head', 'patch', 'trace'];
const FLOW = '/flows/{flowId}';
const PUBLISH = '/flows/{flowId}/publish';
const FLOWS = '/projects/{projectId}/flows';
const SYNC = '/projects/{projectId}/sync';

const only = (doc, path, methods) => {
  for (const m of METHODS) if (!methods.includes(m)) delete doc.paths[path][m];
};

const renameKey = (obj, from, to) => {
  const entries = Object.entries(obj).map(([k, v]) => [k === from ? to : k, v]);
  for (const k of Object.keys(obj)) delete obj[k];
  Object.assign(obj, Object.fromEntries(entries));
};

const without = (list, predicate) => list.filter((item) => !predicate(item));
const refName = (item) => item?.$ref?.split('/').pop();

/**
 * Case table. `keep` lists the path keys left in the document (`webhooks: true` keeps the
 * webhooks), `rule` is the orcon rule the case must trigger, `mutate` plants the defect.
 */
export const CASES = {
  'path-shape': {
    rule: 'path-shape',
    keep: ['/flows/{flowId}/draft'],
    mutate(d) {
      d.paths['/flows/{flowId}/draft']['x-orcon-resource'] = 'item';
    },
  },
  'action-verb': {
    rule: 'action-verb',
    keep: [PUBLISH],
    mutate(d) {
      renameKey(d.paths, PUBLISH, '/flows/{flowId}/launch');
    },
  },
  'path-param-name': {
    rule: 'path-param-name',
    keep: [FLOW],
    mutate(d) {
      only(d, FLOW, ['get']);
      renameKey(d.paths, FLOW, '/flows/{id}');
      d.paths['/flows/{id}'].parameters = [
        {
          name: 'id',
          in: 'path',
          required: true,
          description: 'Id of the flow.',
          schema: { type: 'string', format: 'uuid', examples: ['0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a60'] },
        },
      ];
    },
  },
  'operation-id': {
    rule: 'operation-id',
    keep: [FLOW],
    mutate(d) {
      only(d, FLOW, ['get']);
      d.paths[FLOW].get.operationId = 'flowGet';
    },
  },
  'header-names': {
    rule: 'header-names',
    keep: [FLOW],
    mutate(d) {
      only(d, FLOW, ['get']);
      d.paths[FLOW].get.parameters = [
        {
          name: 'X-Trace-Id',
          in: 'header',
          required: false,
          description: 'Id that correlates the request with a trace.',
          schema: { type: 'string', maxLength: 64, examples: ['trace-1'] },
        },
      ];
    },
  },
  'field-camel-case': {
    rule: 'field-camel-case',
    keep: [FLOW],
    mutate(d) {
      only(d, FLOW, ['get']);
      d.components.schemas.Flow.properties.display_name = { type: 'string', description: 'Display name.', examples: ['Sync'] };
    },
  },
  'id-format': {
    rule: 'id-format',
    keep: [FLOW],
    mutate(d) {
      only(d, FLOW, ['get']);
      d.components.schemas.Flow.properties.ownerId = { type: 'integer', description: 'Id of the owner.', examples: [1] };
    },
  },
  'enum-upper-snake': {
    rule: 'enum-upper-snake',
    keep: [FLOW],
    mutate(d) {
      only(d, FLOW, ['get']);
      d.components.schemas.Flow.properties.status.enum = ['ACTIVE', 'inactive'];
    },
  },
  'time-format': {
    rule: 'time-format',
    keep: [FLOW],
    mutate(d) {
      only(d, FLOW, ['get']);
      delete d.components.schemas.Flow.properties.updatedAt.format;
    },
  },
  'secret-fields': {
    rule: 'secret-fields',
    keep: [FLOW],
    mutate(d) {
      only(d, FLOW, ['get']);
      d.components.schemas.Flow.properties.apiKey = { type: 'string', format: 'password', description: 'Key of the flow.', examples: ['example-secret-1'] };
    },
  },
  'named-schemas': {
    rule: 'named-schemas',
    keep: [FLOWS],
    mutate(d) {
      only(d, FLOWS, ['post']);
      d.paths[FLOWS].post.requestBody.content['application/json'].schema = {
        type: 'object',
        description: 'Fields of a new flow.',
        additionalProperties: false,
        required: ['name'],
        properties: { name: { type: 'string', maxLength: 200, description: 'Display name of the flow.', examples: ['Sync CRM contacts'] } },
      };
    },
  },
  'response-envelope': {
    rule: 'response-shape',
    keep: [FLOW],
    mutate(d) {
      only(d, FLOW, ['get']);
      const media = d.paths[FLOW].get.responses['200'].content['application/json'];
      const flow = media.examples.getExistingFlow.value;
      d.components.schemas.FlowEnvelope = {
        type: 'object',
        description: 'A flow wrapped in an envelope.',
        required: ['data', 'meta', 'success'],
        properties: {
          data: { $ref: '#/components/schemas/Flow' },
          meta: {
            type: 'object',
            description: 'Request metadata.',
            properties: { requestId: { type: 'string', format: 'uuid', description: 'Id of the request.', examples: ['0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a62'] } },
          },
          success: { type: 'boolean', description: 'Whether the request succeeded.' },
        },
      };
      media.schema = { $ref: '#/components/schemas/FlowEnvelope' };
      media.examples.getExistingFlow.value = { data: flow, meta: { requestId: '0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a62' }, success: true };
    },
  },
  'list-pagination': {
    rule: 'list-pagination',
    keep: [FLOWS],
    mutate(d) {
      only(d, FLOWS, ['get']);
      d.paths[FLOWS].get.parameters = without(d.paths[FLOWS].get.parameters, (p) => refName(p) === 'Limit');
    },
  },
  'sort-allowlist': {
    rule: 'sort-allowlist',
    keep: [FLOWS],
    mutate(d) {
      only(d, FLOWS, ['get']);
      const sort = d.paths[FLOWS].get.parameters.find((p) => p.name === 'sort');
      delete sort['x-orcon-sort-default'];
    },
  },
  'problem-responses': {
    rule: 'problem-responses',
    keep: [FLOW],
    mutate(d) {
      only(d, FLOW, ['get']);
      d.paths[FLOW].get.responses['404'] = {
        description: 'The flow does not exist.',
        content: { 'application/json': { schema: { $ref: '#/components/schemas/Problem' } } },
      };
    },
  },
  'problem-codes-declared': {
    rule: 'problem-codes-declared',
    keep: [FLOW],
    mutate(d) {
      only(d, FLOW, ['get']);
      d.paths[FLOW].get['x-orcon-problems'] = without(d.paths[FLOW].get['x-orcon-problems'], (c) => c === 'not-found');
    },
  },
  idempotency: {
    rule: 'idempotency',
    keep: [PUBLISH],
    mutate(d) {
      delete d.paths[PUBLISH].post['x-orcon-idempotency'];
    },
  },
  concurrency: {
    rule: 'concurrency',
    keep: [FLOW],
    mutate(d) {
      only(d, FLOW, ['patch']);
      d.paths[FLOW].patch.parameters = without(d.paths[FLOW].patch.parameters, (p) => refName(p) === 'IfMatch');
    },
  },
  'rate-class': {
    rule: 'rate-class',
    keep: [FLOW],
    mutate(d) {
      only(d, FLOW, ['get']);
      d.paths[FLOW].get['x-orcon-rate-class'] = 'write';
    },
  },
  'long-running': {
    rule: 'long-running',
    keep: [SYNC],
    mutate(d) {
      delete d.paths[SYNC].post.responses['202'].headers.Location;
    },
  },
  stability: {
    rule: 'stability',
    keep: [FLOW],
    mutate(d) {
      only(d, FLOW, ['get']);
      d.paths[FLOW].get['x-orcon-stability'] = 'tested';
    },
  },
  deprecation: {
    rule: 'deprecation',
    keep: [FLOW],
    mutate(d) {
      only(d, FLOW, ['get']);
      d.paths[FLOW].get.deprecated = true;
    },
  },
  permission: {
    rule: 'permission',
    keep: [FLOW],
    mutate(d) {
      only(d, FLOW, ['get']);
      d.paths[FLOW].get['x-orcon-permission'] = 'flow:peek';
    },
  },
  audit: {
    rule: 'audit',
    keep: [FLOW],
    mutate(d) {
      only(d, FLOW, ['patch']);
      d.paths[FLOW].patch['x-orcon-audit'] = 'none';
    },
  },
  'cli-declared': {
    rule: 'cli-declared',
    keep: [FLOW],
    mutate(d) {
      only(d, FLOW, ['get']);
      delete d.paths[FLOW].get['x-orcon-cli'];
    },
  },
  'mcp-declared': {
    rule: 'mcp-declared',
    keep: [PUBLISH],
    mutate(d) {
      d.paths[PUBLISH].post['x-orcon-mcp'].confirm = true;
    },
  },
  'doc-operation': {
    rule: 'doc-operation',
    keep: [FLOW],
    mutate(d) {
      only(d, FLOW, ['get']);
      d.paths[FLOW].get.summary = 'Get a flow.';
    },
  },
  'doc-fields': {
    rule: 'doc-fields',
    keep: [FLOW],
    mutate(d) {
      only(d, FLOW, ['get']);
      delete d.components.schemas.Flow.properties.name.description;
    },
  },
  'doc-examples': {
    rule: 'doc-examples',
    keep: [FLOW],
    mutate(d) {
      only(d, FLOW, ['get']);
      delete d.paths[FLOW].get['x-orcon-examples'];
    },
  },
  events: {
    rule: 'events',
    keep: [],
    webhooks: true,
    mutate(d) {
      d.webhooks['flow.published'].post['x-orcon-event'].cloudEventsType = 'flow.published';
    },
  },
  'codegen-subset': {
    rule: 'codegen-subset',
    keep: [FLOW],
    mutate(d) {
      only(d, FLOW, ['get']);
      d.components.schemas.Flow.unevaluatedProperties = false;
    },
  },
};

const SECTIONS = ['schemas', 'responses', 'parameters', 'headers'];
const REF = /"#\/components\/(schemas|responses|parameters|headers)\/([^"]+)"/g;

/** Removes the components nothing reachable from paths and webhooks references. */
function pruneComponents(doc) {
  const used = new Set();
  const queue = [JSON.stringify({ paths: doc.paths, webhooks: doc.webhooks })];
  while (queue.length > 0) {
    for (const [, section, name] of queue.pop().matchAll(REF)) {
      const key = `${section}/${name}`;
      if (used.has(key) || !doc.components[section]?.[name]) continue;
      used.add(key);
      queue.push(JSON.stringify(doc.components[section][name]));
    }
  }
  for (const section of SECTIONS) {
    for (const name of Object.keys(doc.components[section] ?? {})) {
      if (!used.has(`${section}/${name}`)) delete doc.components[section][name];
    }
    if (Object.keys(doc.components[section] ?? {}).length === 0) delete doc.components[section];
  }
}

function pruneTags(doc) {
  const text = JSON.stringify({ paths: doc.paths, webhooks: doc.webhooks });
  doc.tags = doc.tags.filter((t) => text.includes(`"${t.name}"`));
}

/** The text of every planted case: Map(file name without extension -> yaml). */
export function buildBadCases() {
  const dir = mkdtempSync(join(tmpdir(), 'orcon-bad-cases-'));
  try {
    const out = join(dir, 'good.yaml');
    bundle(join(packageDir, 'testdata', 'good', 'openapi.yaml'), out);
    const good = parse(readFileSync(out, 'utf8'));
    const files = new Map();
    for (const [name, spec] of Object.entries(CASES)) {
      const doc = structuredClone(good);
      doc.paths = Object.fromEntries(spec.keep.map((p) => [p, doc.paths[p]]));
      if (!spec.webhooks) delete doc.webhooks;
      spec.mutate(doc);
      pruneComponents(doc);
      pruneTags(doc);
      doc.info.title = `Planted case: ${name}`;
      doc.info.description = `Derived from testdata/good by test/support/bad-cases.mjs. It must fail with orcon/${spec.rule} and no other orcon rule.`;
      const header = [
        '# SPDX-FileCopyrightText: The Orcon Authors',
        '# SPDX-License-Identifier: Apache-2.0',
        `# Planted case for orcon/${spec.rule}. GENERATED by test/support/write-bad-cases.mjs; do not edit.`,
        '',
      ].join('\n');
      files.set(name, header + stringify(doc, { lineWidth: 0 }));
    }
    return files;
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
}
