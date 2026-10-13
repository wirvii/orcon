// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { isJsonMedia, isObject, isObjectSchema, mergedProperties, typesOf } from '../model.js';
import { defineRule, isError, isSuccess } from '../rule.js';

const ENVELOPE = ['data', 'meta', 'success', 'ok', 'result', 'results', 'payload', 'response', 'error', 'errors', 'pagination', 'links', 'requestId', 'etag'];
const LIST_EXTRAS = ['total', 'totalIsLowerBound'];

export const isList = (entry) => entry.method === 'get' && ['collection', 'view'].includes(entry.item['x-orcon-resource']);

export const responseShape = defineRule((model, report) => {
  for (const entry of model.contractOperations) {
    for (const r of model.responses(entry)) {
      if (r.status === '204') {
        if (isObject(r.resp.content) && Object.keys(r.resp.content).length > 0) report('RESP-5', '204 has no body; remove content.', r.loc.child(['content']));
        continue;
      }
      if (!isSuccess(r.status)) continue;
      for (const mt of model.mediaTypes(r.resp, r.loc)) {
        if (!isJsonMedia(mt.type) || /problem\+json/i.test(mt.type)) continue;
        const schemaLoc = mt.loc.child(['schema']);
        if (!isObject(mt.media.schema)) continue;
        const resolved = model.deref(mt.media.schema, schemaLoc);
        const node = resolved.node;
        if (!isObject(node)) continue;
        if (!isObjectSchema(node) && !Array.isArray(node.allOf)) {
          report('RESP-1', `A ${r.status} JSON body is the resource object or a { items, nextPageToken? } list, not a ${typesOf(node).join('/') || 'bare'} value.`, schemaLoc);
          continue;
        }
        const props = mergedProperties(model, node, resolved.loc);
        const names = props.map((p) => p.name);
        for (const p of props) {
          if (ENVELOPE.includes(p.name)) {
            report('RESP-6', `"${p.name}" is an envelope member at the root of a ${r.status} body; return the resource itself (headers carry request id and ETag).`, p.loc);
          }
          if (p.name === 'status') {
            const s = model.deref(p.schema, p.loc).node;
            if (typesOf(s).includes('boolean')) report('RESP-6', 'A boolean "status" at the root of a body is an envelope member.', p.loc);
          }
        }
        if (names.includes('items')) {
          const extras = names.filter((n) => n !== 'items' && n !== 'nextPageToken' && !LIST_EXTRAS.includes(n));
          for (const n of extras) report('RESP-2', `A list is exactly { items, nextPageToken? }; "${n}" is not allowed.`, resolved.loc);
          if (names.some((n) => LIST_EXTRAS.includes(n)) && entry.op['x-orcon-include-total'] !== true) {
            report('RESP-2', 'total and totalIsLowerBound are allowed only where the operation declares x-orcon-include-total: true.', resolved.loc);
          }
          const itemsProp = props.find((p) => p.name === 'items');
          const items = model.deref(itemsProp.schema, itemsProp.loc);
          const element = isObject(items.node) ? items.node.items : undefined;
          if (!isObject(items.node) || !typesOf(items.node).includes('array') || !isObject(element) || typeof element.$ref !== 'string') {
            report('RESP-2', '"items" is an array of a named schema ($ref).', itemsProp.loc);
          }
        }
      }
    }
  }
});

const LEGACY_PAGING = ['offset', 'page', 'pageSize', 'perPage', 'pageNumber'];

export const listPagination = defineRule((model, report) => {
  for (const entry of model.contractOperations) {
    if (!isList(entry)) continue;
    const params = model.parameters(entry).filter((p) => p.in === 'query');
    const byName = new Map(params.map((p) => [p.name, p]));
    const limit = byName.get('limit');
    if (!limit) {
      report('PAGE-1', `List ${entry.method.toUpperCase()} ${entry.key} takes no "limit" query parameter.`, entry.loc);
    } else {
      const s = model.deref(limit.schema, limit.schemaLoc);
      const n = s.node;
      if (!isObject(n) || !typesOf(n).includes('integer') || n.minimum !== 1 || n.maximum !== 200 || n.default !== 50) {
        report('PAGE-1', '"limit" is an integer with minimum 1, maximum 200 and default 50.', limit.loc);
      }
    }
    if (!byName.has('pageToken')) report('PAGE-1', `List ${entry.method.toUpperCase()} ${entry.key} takes no "pageToken" query parameter.`, entry.loc);
    for (const name of LEGACY_PAGING) {
      if (byName.has(name)) report('PAGE-1', `"${name}" is not allowed; collections use limit and pageToken.`, byName.get(name).loc);
    }
    if (byName.has('includeTotal') && entry.op['x-orcon-include-total'] !== true) {
      report('PAGE-4', 'includeTotal needs x-orcon-include-total: true on the operation.', byName.get('includeTotal').loc);
    }
  }
});

export const sortAllowlist = defineRule((model, report) => {
  for (const entry of model.pathOperations) {
    for (const p of model.parameters(entry)) {
      if (p.in !== 'query' || p.name !== 'sort') continue;
      const s = model.deref(p.schema, p.schemaLoc);
      const values = isObject(s.node) && Array.isArray(s.node.enum) ? s.node.enum : undefined;
      if (!values || values.length === 0) {
        report('PAGE-5', '"sort" lists the allowed expressions in an enum.', p.loc);
      }
      const def = p.param['x-orcon-sort-default'];
      if (typeof def !== 'string') {
        report('PAGE-5', '"sort" declares x-orcon-sort-default.', p.loc);
      } else if (values && !values.includes(def)) {
        report('PAGE-5', `x-orcon-sort-default "${def}" is not one of the allowed sort values.`, p.loc.child(['x-orcon-sort-default']));
      }
    }
  }
});

export const problemResponses = defineRule((model, report) => {
  for (const entry of model.contractOperations) {
    for (const r of model.responses(entry)) {
      if (!isError(r.status)) continue;
      const media = r.resp.content?.['application/problem+json'];
      if (!isObject(media)) {
        report('ERR-1', `Response ${r.status} of ${entry.method.toUpperCase()} ${entry.key} is not application/problem+json.`, r.loc);
        continue;
      }
      const ref = media.schema?.$ref;
      if (typeof ref !== 'string' || !/(^|[/#])Problem(\.ya?ml|\.json)?$/.test(ref)) {
        report('ERR-1', 'The problem+json schema is a $ref to components/schemas/Problem.', r.loc.child(['content', 'application/problem+json', 'schema']));
      }
      const others = Object.keys(r.resp.content ?? {}).filter((t) => t !== 'application/problem+json');
      if (others.length > 0) report('ERR-1', `Error responses carry only application/problem+json (found ${others.join(', ')}).`, r.loc.child(['content']));
    }
  }
});

const MIN_CODE_REASON = {
  unauthenticated: 'every authenticated operation',
  'rate-limited': 'every authenticated operation',
  'internal-error': 'every authenticated operation',
  forbidden: 'an operation with a permission other than authenticated',
  'not-found': 'an operation with an id in the path',
  'validation-failed': 'an operation with parameters or a body',
  'payload-too-large': 'an operation with a body',
  'idempotency-key-reused': 'an operation with idempotency',
  'idempotency-in-progress': 'an operation with idempotency',
  'precondition-failed': 'an operation with If-Match',
  'precondition-required': 'an operation with If-Match',
  'invalid-page-token': 'a list',
  'experimental-not-enabled': 'an experimental operation',
};

function minimumCodes(model, entry) {
  const need = new Set();
  const perm = entry.op['x-orcon-permission'];
  if (perm !== 'anonymous') ['unauthenticated', 'rate-limited', 'internal-error'].forEach((c) => need.add(c));
  if (perm !== undefined && perm !== 'authenticated' && perm !== 'anonymous') need.add('forbidden');
  const params = model.parameters(entry);
  if (params.some((p) => p.in === 'path')) need.add('not-found');
  const hasBody = model.requestBody(entry) !== undefined;
  if (hasBody || params.some((p) => p.in === 'path' || p.in === 'query')) need.add('validation-failed');
  if (hasBody) need.add('payload-too-large');
  if (entry.op['x-orcon-idempotency'] !== undefined) {
    need.add('idempotency-key-reused');
    need.add('idempotency-in-progress');
  }
  if (params.some((p) => p.in === 'header' && p.name.toLowerCase() === 'if-match')) {
    need.add('precondition-failed');
    need.add('precondition-required');
  }
  if (isList(entry)) need.add('invalid-page-token');
  if (entry.op['x-orcon-stability'] === 'experimental') need.add('experimental-not-enabled');
  return need;
}

export const problemCodesDeclared = defineRule((model, report) => {
  const ops = model.contractOperations;
  if (ops.length === 0) return;
  const schema = model.namedSchema('ProblemCode');
  const catalog = schema && isObject(schema.node) ? schema.node['x-orcon-problem-catalog'] : undefined;
  if (!isObject(catalog)) {
    report('ERR-6', 'components/schemas/ProblemCode with x-orcon-problem-catalog is the problem code catalog; it is missing.', model.rootLoc);
    return;
  }
  const catalogLoc = schema.loc.child(['x-orcon-problem-catalog']);
  const enumValues = Array.isArray(schema.node.enum) ? schema.node.enum : [];
  for (const [code, info] of Object.entries(catalog)) {
    const l = catalogLoc.child([code]);
    if (!isObject(info) || !Number.isInteger(info.status) || info.status < 400 || info.status > 599 || typeof info.title !== 'string' || typeof info.when !== 'string' || typeof info.retryable !== 'boolean') {
      report('ERR-6', `Catalog entry "${code}" needs status (400-599), title, when and retryable (boolean).`, l);
    }
    if (!enumValues.includes(code)) report('ERR-7', `Catalog code "${code}" is missing from the ProblemCode enum.`, schema.loc.child(['enum']));
  }
  for (const code of enumValues) {
    if (!(code in catalog)) report('ERR-7', `ProblemCode value "${code}" has no catalog entry.`, schema.loc.child(['enum']));
  }
  for (const entry of ops) {
    const declared = entry.op['x-orcon-problems'];
    const where = entry.loc.child(['x-orcon-problems']);
    if (!Array.isArray(declared)) {
      report('ERR-3', `${entry.method.toUpperCase()} ${entry.key} declares no x-orcon-problems.`, entry.loc);
      continue;
    }
    const responses = model.responses(entry);
    const hasStatus = (status) => responses.some((r) => r.status === String(status) || r.status === `${Math.floor(status / 100)}XX`);
    declared.forEach((code, i) => {
      const l = where.child([i]);
      if (!(code in catalog)) {
        report('ERR-3', `Problem code "${code}" is not in the catalog (ERR-7: new codes are added to the catalog first).`, l);
        return;
      }
      const status = catalog[code]?.status;
      if (Number.isInteger(status) && !hasStatus(status)) {
        report('ERR-3', `Problem code "${code}" has status ${status} but the operation declares no ${status} response.`, l);
      }
    });
    for (const code of minimumCodes(model, entry)) {
      if (!declared.includes(code)) {
        report('ERR-4', `Problem code "${code}" is required for ${MIN_CODE_REASON[code]} and is not in x-orcon-problems.`, where);
      }
    }
  }
});
