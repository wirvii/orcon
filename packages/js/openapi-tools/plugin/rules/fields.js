// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { isJsonMedia, isObject, isObjectSchema, refName, typesOf } from '../model.js';
import { CAMEL, UPPER_SNAKE, defineRule } from '../rule.js';

const SECRET_EXAMPLE = /^example-secret-[0-9]+$/;
const SENSITIVE_EXACT = /^(secret|password|passphrase|token|apiKey|privateKey|accessToken|refreshToken|clientSecret)$/;
const SENSITIVE_SUFFIX = /(Secret|Password|Passphrase|PrivateKey|AccessToken|RefreshToken|ApiKey)$/;

const isSensitiveName = (name) => SENSITIVE_EXACT.test(name) || SENSITIVE_SUFFIX.test(name);

/** Calls fn(name, rawProperty, resolved, loc) for every property of every schema once. */
function eachProperty(model, fn) {
  for (const { schema, loc } of model.allSchemas) {
    if (!isObject(schema.properties)) continue;
    for (const name of Object.keys(schema.properties)) {
      const raw = schema.properties[name];
      const propLoc = loc.child(['properties', name]);
      const resolved = model.deref(raw, propLoc);
      fn(name, raw, resolved, propLoc);
    }
  }
}

function eachQueryParam(model, fn) {
  for (const entry of model.operations) {
    for (const p of model.parameters(entry)) {
      if (p.in !== 'query' || !p.schema) continue;
      fn(p.name, p, model.deref(p.schema, p.schemaLoc), p.loc);
    }
  }
}

export const fieldCamelCase = defineRule((model, report) => {
  eachProperty(model, (name, raw, resolved, loc) => {
    if (!CAMEL.test(name)) report('FIELD-1', `Property "${name}" is not camelCase.`, loc);
  });
  for (const entry of model.operations) {
    for (const p of model.parameters(entry)) {
      if (p.in === 'query' && !CAMEL.test(p.name)) report('FIELD-1', `Query parameter "${p.name}" is not camelCase.`, p.loc);
    }
  }
});

const isUuid = (schema) => isObject(schema) && typesOf(schema).includes('string') && schema.format === 'uuid';

export const idFormat = defineRule((model, report) => {
  const check = (name, raw, resolved, loc) => {
    const isId = name === 'id' || /[a-z0-9]Id$/.test(name);
    const isIds = /[a-z0-9]Ids$/.test(name);
    if (!isId && !isIds) return;
    if (raw?.['x-orcon-external-id'] === true || resolved.node?.['x-orcon-external-id'] === true) return;
    const node = resolved.node;
    if (isId && !isUuid(node)) {
      report('FIELD-2', `"${name}" must be a string with format: uuid (provider ids declare x-orcon-external-id: true).`, loc);
    }
    if (isIds) {
      const items = isObject(node) && typesOf(node).includes('array') ? model.deref(node.items, resolved.loc.child(['items'])).node : undefined;
      if (!isUuid(items)) report('FIELD-2', `"${name}" must be an array of uuid strings.`, loc);
    }
  };
  eachProperty(model, check);
  eachQueryParam(model, (name, p, resolved, loc) => check(name, p.param, resolved, loc));
});

export const enumUpperSnake = defineRule((model, report) => {
  for (const { schema, loc } of model.allSchemas) {
    if (!Array.isArray(schema.enum)) continue;
    if (schema['x-orcon-enum-kind'] === 'identifier') continue;
    for (const [i, value] of schema.enum.entries()) {
      if (typeof value === 'string' && !UPPER_SNAKE.test(value)) {
        report('FIELD-3', `Enum value "${value}" is not UPPER_SNAKE; identifier catalogs declare x-orcon-enum-kind: identifier.`, loc.child(['enum', i]));
      }
    }
  }
});

const DATE_TIME_OK = /(At|After|Before)$|^(since|until|time)$/;
const DURATION_NAME = /(Duration|Timeout|Interval|Ttl|Window)$/;

export const timeFormat = defineRule((model, report) => {
  const check = (name, raw, resolved, loc) => {
    const node = resolved.node;
    if (!isObject(node)) return;
    const format = node.format;
    if (/[a-z0-9]At$/.test(name) && !(typesOf(node).includes('string') && format === 'date-time')) {
      report('FIELD-4', `"${name}" is an instant: use type: string with format: date-time.`, loc);
    }
    if (format === 'date-time' && !DATE_TIME_OK.test(name)) {
      report('FIELD-4', `"${name}" has format: date-time but is not named *At, since or until.`, loc);
    }
    if (/[a-z0-9]Date$/.test(name) && !(typesOf(node).includes('string') && format === 'date')) {
      report('FIELD-4', `"${name}" is a date: use type: string with format: date.`, loc);
    }
    if (DURATION_NAME.test(name) && typesOf(node).includes('string') && format !== 'duration') {
      report('FIELD-4', `"${name}" is a duration: use an ISO 8601 string with format: duration.`, loc);
    }
    if (/[a-z0-9]Bytes$/.test(name) && !typesOf(node).includes('integer')) {
      report('FIELD-4', `"${name}" is a size: use type: integer.`, loc);
    }
  };
  eachProperty(model, check);
  eachQueryParam(model, (name, p, resolved, loc) => check(name, p.param, resolved, loc));
});

function checkExampleValue(value, loc, report) {
  if (Array.isArray(value)) {
    value.forEach((v, i) => checkExampleValue(v, loc.child([i]), report));
  } else if (isObject(value)) {
    for (const [key, v] of Object.entries(value)) {
      if (typeof v === 'string' && isSensitiveName(key) && !SECRET_EXAMPLE.test(v)) {
        report('FIELD-7', `Example value of secret field "${key}" must be a marker like example-secret-1.`, loc.child([key]));
      } else {
        checkExampleValue(v, loc.child([key]), report);
      }
    }
  }
}

/**
 * FIELD-7: schemas of `<Resource>WithSecret` response bodies, mapped to the operations that
 * return them. The only returned secret is their readOnly `secret` property.
 */
function withSecretHolders(model) {
  const holders = new Map();
  for (const entry of model.contractOperations) {
    for (const r of model.responses(entry)) {
      if (!/^2/.test(r.status)) continue;
      for (const mt of model.mediaTypes(r.resp, r.loc)) {
        const raw = mt.media.schema;
        const name = refName(raw);
        if (!name || !/WithSecret$/.test(name)) continue;
        const top = model.deref(raw, mt.loc.child(['schema']));
        const nodes = [top.node];
        if (isObject(top.node) && Array.isArray(top.node.allOf)) {
          // Only inline members: a member that is a $ref (the base resource) is shared with
          // other responses and never holds a returned secret.
          top.node.allOf.forEach((member, i) => {
            if (isObject(member) && typeof member.$ref !== 'string') nodes.push(model.deref(member, top.loc.child(['allOf', i])).node);
          });
        }
        for (const node of nodes) {
          if (!isObject(node)) continue;
          if (!holders.has(node)) holders.set(node, { name, entries: [] });
          holders.get(node).entries.push(entry);
        }
      }
    }
  }
  return holders;
}

/** Why the readOnly `secret` property of `holder` is not an allowed returned secret, or undefined. */
function returnedSecretProblem(node, holder) {
  if (node.readOnly !== true || node.format !== 'password') return 'it is readOnly: true with format: password';
  if (!holder) return 'it belongs to a response schema named <Resource>WithSecret';
  const bad = holder.entries.find((e) => e.op['x-orcon-idempotency'] !== 'required' || !(e.op['x-orcon-problems'] ?? []).includes('secret-already-issued'));
  if (bad) return `${bad.op.operationId ?? bad.key} that returns it declares x-orcon-idempotency: required and secret-already-issued`;
  return undefined;
}

export const secretFields = defineRule((model, report) => {
  const holders = withSecretHolders(model);
  for (const { schema, loc } of model.allSchemas) {
    if (!isObject(schema.properties)) continue;
    for (const name of Object.keys(schema.properties)) {
      const raw = schema.properties[name];
      const propLoc = loc.child(['properties', name]);
      const resolved = model.deref(raw, propLoc);
      const node = resolved.node;
      if (!isObject(node)) continue;
      const sensitive = node.format === 'password' || (typesOf(node).includes('string') && isSensitiveName(name));
      if (!sensitive) continue;
      const writeOnly = node.writeOnly === true || raw?.writeOnly === true;
      if (writeOnly && (node.readOnly === true || raw?.readOnly === true)) {
        report('FIELD-7', `Secret field "${name}" is both writeOnly and readOnly; a secret is either received (writeOnly) or returned once (readOnly in a <Resource>WithSecret).`, propLoc);
      }
      if (!writeOnly) {
        const returned = name === 'secret' && (node.readOnly === true || node.format === 'password');
        if (!returned) {
          report('FIELD-7', `Secret field "${name}" must set writeOnly: true and is never returned (the only exception is the readOnly "secret" of a <Resource>WithSecret response).`, propLoc);
        } else {
          const why = returnedSecretProblem(node, holders.get(schema));
          if (why) report('FIELD-7', `A returned secret is allowed only as the readOnly "secret" property of a <Resource>WithSecret response, once, in an operation with required idempotency and secret-already-issued; check that ${why}.`, propLoc);
        }
      }
      const samples = [node.example, ...(Array.isArray(node.examples) ? node.examples : []), node.default];
      samples.forEach((sample) => {
        if (typeof sample === 'string' && !SECRET_EXAMPLE.test(sample)) {
          report('FIELD-7', `Example of secret field "${name}" must be a marker like example-secret-1.`, propLoc);
        }
      });
    }
  }
  // A password-format schema reached through `items` or `additionalProperties` is a secret
  // too; it is allowed only as writeOnly (it cannot be the single `secret` property).
  for (const { schema, loc } of model.allSchemas) {
    for (const key of ['items', 'additionalProperties']) {
      if (!isObject(schema[key])) continue;
      const child = model.deref(schema[key], loc.child([key]));
      if (isObject(child.node) && child.node.format === 'password' && child.node.writeOnly !== true && schema[key].writeOnly !== true) {
        report('FIELD-7', `A secret in "${key}" must set writeOnly: true; a returned secret is only the "secret" property of a <Resource>WithSecret response.`, child.loc);
      }
    }
  }
  for (const entry of model.operations) {
    const holdersOfExamples = [];
    const body = model.requestBody(entry);
    if (body) holdersOfExamples.push(...model.mediaTypes(body.node, body.loc));
    for (const r of model.responses(entry)) holdersOfExamples.push(...model.mediaTypes(r.resp, r.loc));
    for (const { media, loc } of holdersOfExamples) {
      if (media.example !== undefined) checkExampleValue(media.example, loc.child(['example']), report);
      if (isObject(media.examples)) {
        for (const name of Object.keys(media.examples)) {
          const ex = model.deref(media.examples[name], loc.child(['examples', name]));
          if (isObject(ex.node) && ex.node.value !== undefined) checkExampleValue(ex.node.value, ex.loc.child(['value']), report);
        }
      }
    }
  }
});

export const namedSchemas = defineRule((model, report) => {
  // FIELD-9: bodies are named schemas.
  for (const entry of model.operations) {
    const holders = [];
    const body = model.requestBody(entry);
    if (body) holders.push(...model.mediaTypes(body.node, body.loc));
    for (const r of model.responses(entry)) holders.push(...model.mediaTypes(r.resp, r.loc));
    for (const { type, media, loc } of holders) {
      if (!isJsonMedia(type) || !isObject(media.schema)) continue;
      if (typeof media.schema.$ref === 'string') continue;
      if (isObjectSchema(media.schema) || Array.isArray(media.schema.allOf) || Array.isArray(media.schema.oneOf) || Array.isArray(media.schema.anyOf)) {
        report('FIELD-9', 'Request and response bodies are named schemas in components/schemas; this body declares an inline object.', loc.child(['schema']));
      }
    }
  }
  // FIELD-9: request schemas are closed. FIELD-10: inputs are bounded.
  // Known limit: a request schema composed with allOf is not checked for
  // additionalProperties: false (closing members of an allOf breaks JSON Schema validation,
  // and unevaluatedProperties is not supported by the Go generator, VER-9). Request bodies
  // should be plain objects; review.md asks reviewers to look at allOf request bodies.
  for (const { schema, loc } of model.requestSchemas()) {
    if (isObjectSchema(schema) && schema.additionalProperties !== false && !isObject(schema.additionalProperties) && !Array.isArray(schema.allOf)) {
      report('FIELD-9', 'Request schemas set additionalProperties: false.', loc);
    }
    checkBounds(schema, loc, report);
  }
  for (const entry of model.operations) {
    for (const p of model.parameters(entry)) {
      if (p.in === 'header' || !p.schema) continue;
      const s = model.deref(p.schema, p.schemaLoc);
      if (isObject(s.node)) checkBounds(s.node, s.loc, report);
    }
  }
});

function checkBounds(schema, loc, report) {
  const types = typesOf(schema);
  if (types.includes('string') && schema.maxLength === undefined && schema.format === undefined && schema.enum === undefined && schema.pattern === undefined && schema.const === undefined) {
    report('FIELD-10', 'Input strings set maxLength, format, enum or pattern.', loc);
  }
  if (types.includes('array') && schema.maxItems === undefined && schema.enum === undefined) {
    report('FIELD-10', 'Request arrays set maxItems.', loc);
  }
}
