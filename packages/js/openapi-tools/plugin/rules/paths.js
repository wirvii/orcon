// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { isObject } from '../model.js';
import { CAMEL, KEBAB, STANDARD_HEADERS, defineRule, lastLiteral } from '../rule.js';
import { CRUD_VERBS, VERBS } from '../verbs.js';

const RESOURCES = ['collection', 'item', 'singleton', 'view', 'action', 'stream'];
const NAMESPACES = ['instance', 'catalog'];

/** Path items of the grammar rules, one entry per path key. */
function pathItems(model) {
  const byKey = new Map();
  for (const entry of model.contractOperations) {
    if (!byKey.has(entry.key)) byKey.set(entry.key, { key: entry.key, item: entry.item, itemLoc: entry.itemLoc, ops: [] });
    byKey.get(entry.key).ops.push(entry);
  }
  return [...byKey.values()];
}

/** "/projects/{projectId}/flows" -> pattern "LPL" after the namespace is removed. */
function shapeOf(path) {
  const segments = path.split('/').slice(1);
  let namespace;
  let rest = segments;
  if (NAMESPACES.includes(segments[0])) {
    namespace = segments[0];
    rest = segments.slice(1);
  } else if (segments[0] === 'users' && segments[1] === 'me') {
    namespace = 'users/me';
    rest = segments.slice(2);
  }
  const pattern = rest.map((s) => (s.startsWith('{') && s.endsWith('}') ? 'P' : 'L')).join('');
  return { segments, rest, namespace, pattern };
}

function isPlural(literal) {
  const word = literal.split('-').pop();
  if (/(ss|us|is)$/.test(word)) return false;
  return word.endsWith('s');
}

const ALLOWED = {
  collection: (shape) => ['L', 'LPL'].includes(shape.pattern),
  view: (shape) => ['L', 'LPL'].includes(shape.pattern),
  item: (shape) => ['LP', 'LPLP'].includes(shape.pattern),
  singleton: (shape) => ['LPL', 'LPLPL'].includes(shape.pattern) || (shape.namespace === 'instance' && shape.pattern === 'L') || (shape.namespace === 'users/me' && shape.pattern === ''),
  action: (shape) => /^(LP){1,2}L{1,2}$/.test(shape.pattern),
  stream: (shape) => shape.pattern.endsWith('L') && ['L', 'LPL'].includes(shape.pattern.slice(0, -1)),
};

const SHAPE_HINT = {
  collection: '/{collection} or /{parent}/{parentId}/{collection}',
  view: '/{collection} or /{parent}/{parentId}/{collection}',
  item: '/{collection}/{collectionId} or /{parent}/{parentId}/{collection}/{collectionId}',
  singleton: '<item>/{singleton}, /users/me or /instance/{singleton}',
  action: '<item or singleton>/{verb}',
  stream: '<collection>/stream',
};

export const pathShape = defineRule((model, report) => {
  // The protocol tag exempts an operation from the rules only on the paths ADR-0052 allows.
  for (const entry of model.pathOperations) {
    if (entry.taggedProtocol && !model.protocolAllowed(entry.key)) {
      report('PATH-1', `${entry.method.toUpperCase()} ${entry.key} is tagged protocol but ${entry.key} is not an allowed protocol path (${model.protocolPaths.join(', ')}); it follows the grammar and every rule.`, entry.loc.child(['tags']));
    }
  }
  for (const { key, item, itemLoc, ops } of pathItems(model)) {
    const shape = shapeOf(key);
    const params = shape.segments.filter((s) => s.startsWith('{'));
    if (params.length > 2) report('PATH-2', `Path ${key} has ${params.length} path parameters; at most two are allowed.`, itemLoc);
    if (key.length > 1 && key.endsWith('/')) report('PATH-2', `Path ${key} ends with a slash.`, itemLoc);
    for (const segment of shape.segments) {
      if (segment.startsWith('{')) continue;
      if (!KEBAB.test(segment)) report('PATH-3', `Path segment "${segment}" of ${key} is not kebab-case.`, itemLoc);
    }
    const resource = item['x-orcon-resource'];
    if (resource === undefined) {
      report('PATH-2', `Path ${key} has no x-orcon-resource (one of ${RESOURCES.join(', ')}).`, itemLoc);
      continue;
    }
    if (!RESOURCES.includes(resource)) {
      report('PATH-2', `Path ${key} has x-orcon-resource "${resource}"; use one of ${RESOURCES.join(', ')}.`, itemLoc.child(['x-orcon-resource']));
      continue;
    }
    if (!ALLOWED[resource](shape)) {
      report('PATH-2', `Path ${key} is not a valid ${resource}; the grammar is ${SHAPE_HINT[resource]}.`, itemLoc.child(['x-orcon-resource']));
      continue;
    }
    const exception = item['x-orcon-plural-exception'];
    const hasException = typeof exception === 'string' && exception.trim() !== '';
    if (!hasException) {
      shape.rest.forEach((segment, i) => {
        if (segment.startsWith('{')) return;
        const next = shape.rest[i + 1];
        const followedByParam = next !== undefined && next.startsWith('{');
        const isCollectionName = resource === 'collection' && i === shape.rest.length - 1;
        if ((followedByParam || isCollectionName) && !isPlural(segment)) {
          report('PATH-3', `Collection segment "${segment}" of ${key} is not plural; use a plural noun or declare x-orcon-plural-exception with the reason.`, itemLoc);
        }
      });
    }
    const last = lastLiteral(key);
    if (['collection', 'view', 'singleton', 'item'].includes(resource) && last && VERBS.includes(last) && ALLOWED.action(shape)) {
      report('PATH-2', `Path ${key} ends in the action verb "${last}" but is declared ${resource}; declare it as an action.`, itemLoc.child(['x-orcon-resource']));
    }
    if (resource === 'stream' && last !== 'stream') report('PATH-2', `A stream path ends in /stream; ${key} does not.`, itemLoc.child(['x-orcon-resource']));
    for (const op of ops) {
      if ((resource === 'view' || resource === 'stream') && op.method !== 'get') {
        report('PATH-2', `A ${resource} path allows GET only; ${key} declares ${op.method.toUpperCase()}.`, op.loc);
      }
    }
  }
});

export const actionVerb = defineRule((model, report) => {
  for (const { key, item, itemLoc, ops } of pathItems(model)) {
    if (item['x-orcon-resource'] !== 'action') continue;
    const last = lastLiteral(key);
    if (!last || !VERBS.includes(last)) {
      report('PATH-5', `Action path ${key} ends in "${last ?? '{parameter}'}", which is not one of the ${VERBS.length} action verbs (${VERBS.join(', ')}).`, itemLoc);
    }
    for (const op of ops) {
      if (op.method !== 'post') report('PATH-5', `An action is POST only; ${key} declares ${op.method.toUpperCase()}.`, op.loc);
    }
  }
});

export const pathParamName = defineRule((model, report) => {
  for (const { key, itemLoc, ops } of pathItems(model)) {
    const segments = key.split('/').slice(1);
    segments.forEach((segment, i) => {
      if (!segment.startsWith('{')) return;
      const name = segment.slice(1, -1);
      const previous = segments[i - 1];
      const isVersion = name === 'version' && previous === 'versions';
      if (!isVersion && !(CAMEL.test(name) && name.length > 2 && name.endsWith('Id'))) {
        report('PATH-6', `Path parameter {${name}} of ${key} must be <resource>Id in camelCase ({flowId}, never {id}), or {version} under versions.`, itemLoc);
        return;
      }
      for (const op of ops) {
        const param = model.parameters(op).find((p) => p.in === 'path' && p.name === name);
        if (!param) continue;
        const schema = model.deref(param.schema, param.schemaLoc).node;
        if (isVersion) continue;
        if (!isObject(schema) || schema.format !== 'uuid') {
          report('PATH-6', `Path parameter ${name} must be a string with format: uuid.`, param.loc);
        }
      }
    });
  }
});

export const operationId = defineRule((model, report) => {
  const allowed = new Set([...CRUD_VERBS, ...VERBS]);
  for (const entry of model.contractOperations) {
    const id = entry.op.operationId;
    if (typeof id !== 'string' || id === '') {
      report('PATH-7', `Operation ${entry.method.toUpperCase()} ${entry.key} has no operationId.`, entry.loc);
      continue;
    }
    if (!CAMEL.test(id)) {
      report('PATH-7', `operationId "${id}" is not lowerCamel.`, entry.loc.child(['operationId']));
      continue;
    }
    const first = id.match(/^[a-z][a-z0-9]*/)[0];
    if (!allowed.has(first)) {
      report('PATH-7', `operationId "${id}" must start with a verb: ${[...CRUD_VERBS].join(', ')} or an action verb.`, entry.loc.child(['operationId']));
    }
  }
});

export const headerNames = defineRule((model, report) => {
  const standard = new Set(STANDARD_HEADERS);
  const check = (name, loc) => {
    const lower = name.toLowerCase();
    if (standard.has(lower)) return;
    if (/^x-/i.test(name)) {
      report('PATH-8', `Header ${name} uses the X- prefix; custom headers use the Orcon- prefix.`, loc);
    } else if (!/^Orcon-[A-Z][A-Za-z0-9]*(-[A-Z][A-Za-z0-9]*)*$/.test(name)) {
      report('PATH-8', `Header ${name} is not a standard header and does not use the Orcon- prefix (for example Orcon-Request-Id).`, loc);
    }
  };
  for (const entry of model.operations) {
    for (const p of model.parameters(entry)) if (p.in === 'header') check(p.name, p.loc);
    for (const r of model.responses(entry)) {
      for (const h of model.responseHeaders(r.resp, r.loc)) check(h.name, h.loc);
    }
  }
  const headers = model.components.node.headers;
  if (isObject(headers)) {
    for (const name of Object.keys(headers)) {
      if (!standard.has(name.toLowerCase()) && /^x-/i.test(name)) check(name, model.components.loc.child(['headers', name]));
    }
  }
});
