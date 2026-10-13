// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { isJsonMedia, isObject, typesOf } from '../model.js';
import { CAMEL, KEBAB, STABILITIES, defineRule } from '../rule.js';

const GENERATED_SECTION = /^\s*(#{1,6}\s*|\*\*|[-*]\s+)?(required permission|rate class|stability|idempotency|problem codes|cli command|cli|mcp tool|mcp)\b[^\n]{0,40}(:|\*\*|$)/im;

function sentences(text) {
  return text
    .split(/[.!?](?:\s|$)/)
    .map((s) => s.trim())
    .filter((s) => s.length > 0);
}

export const docOperation = defineRule((model, report) => {
  for (const entry of model.operations) {
    const label = entry.kind === 'webhook' ? `Webhook ${entry.key}` : `${entry.method.toUpperCase()} ${entry.key}`;
    const { summary, description } = entry.op;
    if (typeof summary !== 'string' || summary.trim() === '') {
      report('DOC-1', `${label} has no summary.`, entry.loc);
    } else {
      if (summary.length > 80) report('DOC-1', `The summary of ${label} is ${summary.length} characters; the limit is 80.`, entry.loc.child(['summary']));
      if (!/^[A-Z]/.test(summary)) report('DOC-1', `The summary of ${label} starts with an uppercase verb ("Publish a flow draft").`, entry.loc.child(['summary']));
      if (/\.$/.test(summary.trim())) report('DOC-1', `The summary of ${label} has no final period.`, entry.loc.child(['summary']));
    }
    if (typeof description !== 'string' || description.trim() === '') {
      report('DOC-2', `${label} has no description.`, entry.loc);
    } else {
      if (description.trim().length < 80 || sentences(description).length < 2) {
        report('DOC-2', `The description of ${label} says what it does and when to use it in at least two sentences (80 characters or more).`, entry.loc.child(['description']));
      }
      if (GENERATED_SECTION.test(description)) {
        report('DOC-3', `The description of ${label} writes by hand a section the reference site generates (permission, rate class, stability, idempotency, problem codes, CLI, MCP).`, entry.loc.child(['description']));
      }
    }
  }
});

const PRIMITIVES = ['string', 'integer', 'number'];

export const docFields = defineRule((model, report) => {
  const needsExample = (node, raw) => {
    const types = typesOf(node);
    if (!types.some((t) => PRIMITIVES.includes(t)) || types.includes('boolean')) return false;
    const has = (n) => isObject(n) && (n.examples !== undefined || n.example !== undefined || n.enum !== undefined || n.const !== undefined);
    return !has(node) && !has(raw);
  };
  for (const { schema, loc } of model.allSchemas) {
    if (!isObject(schema.properties)) continue;
    for (const name of Object.keys(schema.properties)) {
      const raw = schema.properties[name];
      const propLoc = loc.child(['properties', name]);
      const resolved = model.deref(raw, propLoc);
      if (!isObject(resolved.node)) continue;
      const described = (typeof raw?.description === 'string' && raw.description.trim() !== '') || (typeof resolved.node.description === 'string' && resolved.node.description.trim() !== '');
      if (!described) report('DOC-5', `Property "${name}" has no description.`, propLoc);
      if (needsExample(resolved.node, raw)) report('DOC-6', `Property "${name}" is a primitive without examples or an enum.`, propLoc);
    }
  }
  for (const entry of model.operations) {
    for (const p of model.parameters(entry)) {
      if (typeof p.param.description !== 'string' || p.param.description.trim() === '') {
        report('DOC-5', `Parameter "${p.name}" has no description.`, p.loc);
      }
      if (p.schema) {
        const s = model.deref(p.schema, p.schemaLoc);
        if (isObject(s.node) && needsExample(s.node, p.schema)) report('DOC-6', `Parameter "${p.name}" is a primitive without examples or an enum.`, p.loc);
      }
    }
  }
});

const SCENARIO_WITHIN = /^([1-9]|10)s$/;

function exampleNames(model, container, containerLoc) {
  const names = new Set();
  for (const mt of model.mediaTypes(container, containerLoc)) {
    if (isObject(mt.media.examples)) Object.keys(mt.media.examples).forEach((n) => names.add(n));
  }
  return names;
}

export const docExamples = defineRule((model, report) => {
  const allNames = new Set();
  for (const entry of model.contractOperations) {
    const list = entry.op['x-orcon-examples'];
    if (Array.isArray(list)) list.forEach((e) => isObject(e) && typeof e.name === 'string' && allNames.add(e.name));
  }
  for (const entry of model.contractOperations) {
    const stability = entry.op['x-orcon-stability'];
    if (stability !== 'stable' && stability !== 'beta') continue;
    const list = entry.op['x-orcon-examples'];
    const where = entry.loc.child(['x-orcon-examples']);
    if (!Array.isArray(list) || list.length === 0) {
      report('DOC-7', `${entry.method.toUpperCase()} ${entry.key} is ${stability} and declares no x-orcon-examples.`, entry.loc);
      continue;
    }
    if (!list.some((e) => isObject(e) && /^2\d\d$/.test(String(e.response?.status)))) {
      report('DOC-7', 'x-orcon-examples has at least one 2xx case.', where);
    }
    const body = model.requestBody(entry);
    const requestNames = body ? exampleNames(model, body.node, body.loc) : new Set();
    const responses = model.responses(entry);
    list.forEach((example, i) => {
      const loc = where.child([i]);
      if (!isObject(example)) {
        report('DOC-9', 'An x-orcon-examples entry is an object.', loc);
        return;
      }
      if (typeof example.name !== 'string' || !CAMEL.test(example.name)) report('DOC-9', 'An example has a lowerCamel name.', loc.child(['name']));
      if (typeof example.scenario !== 'string' || !KEBAB.test(example.scenario)) report('DOC-9', `Example "${example.name}" names a kebab-case scenario.`, loc.child(['scenario']));
      if (!isObject(example.request)) report('DOC-9', `Example "${example.name}" has a request.`, loc);
      const status = example.response?.status;
      if (!Number.isInteger(status)) {
        report('DOC-9', `Example "${example.name}" has a response.status.`, loc);
      } else {
        const response = responses.find((r) => r.status === String(status));
        if (!response) report('DOC-7', `Example "${example.name}" expects status ${status} but the operation declares no such response.`, loc.child(['response']));
        const bodyName = example.response.body;
        if (bodyName !== undefined && response && !exampleNames(model, response.resp, response.loc).has(bodyName)) {
          report('DOC-9', `Example "${example.name}" names the response example "${bodyName}", which the ${status} response does not define.`, loc.child(['response', 'body']));
        }
      }
      const requestBodyName = example.request?.body;
      if (requestBodyName !== undefined && !requestNames.has(requestBodyName)) {
        report('DOC-9', `Example "${example.name}" names the request example "${requestBodyName}", which the request body does not define.`, loc.child(['request', 'body']));
      }
      if (example.stream !== undefined) {
        const s = example.stream;
        if (!isObject(s) || typeof s.trigger !== 'string' || !Array.isArray(s.expectTypes) || !SCENARIO_WITHIN.test(String(s.within))) {
          report('DOC-9', 'A stream example has trigger, expectTypes and within (at most 10s).', loc.child(['stream']));
        } else if (!allNames.has(s.trigger)) {
          report('DOC-9', `Stream trigger "${s.trigger}" is not the name of an example.`, loc.child(['stream', 'trigger']));
        }
      }
    });
  }
});

const EVENT_TYPE = /^[a-z][a-z0-9]*(_[a-z0-9]+)*\.[a-z][a-z0-9]*(_[a-z0-9]+)*$/;
const CE_TYPE = /^dev\.orcon\.([a-z][a-z0-9_]*)\.([a-z][a-z0-9_]*)\.v[1-9][0-9]*$/;
const EXECUTION_DATA = /^(inputData|outputData|executionData|runData|nodeData|resultData|input|output)$/;

export const events = defineRule((model, report) => {
  const starts = [];
  for (const entry of model.operations) {
    if (entry.kind !== 'webhook') continue;
    const event = entry.op['x-orcon-event'];
    const where = entry.loc.child(['x-orcon-event']);
    if (!isObject(event)) {
      report('EVT-2', `Webhook ${entry.key} declares x-orcon-event { type, cloudEventsType, permission, stability, since }.`, entry.loc);
    } else {
      if (typeof event.type !== 'string' || !EVENT_TYPE.test(event.type)) {
        report('EVT-1', 'An event type is <area>.<event> in lowercase snake segments.', where.child(['type']));
      } else if (event.type !== entry.key) {
        report('EVT-2', `The webhooks key "${entry.key}" is the event type "${event.type}".`, where.child(['type']));
      }
      const ce = typeof event.cloudEventsType === 'string' ? CE_TYPE.exec(event.cloudEventsType) : null;
      if (!ce || (typeof event.type === 'string' && `${ce[1]}.${ce[2]}` !== event.type)) {
        report('EVT-1', 'cloudEventsType is dev.orcon.<area>.<event>.v<N> and repeats the type.', where.child(['cloudEventsType']));
      }
      if (typeof event.permission !== 'string' || event.permission === '') report('EVT-2', 'x-orcon-event.permission names the permission that receives the event.', where);
      if (!STABILITIES.includes(event.stability)) report('EVT-2', `x-orcon-event.stability is one of ${STABILITIES.join(', ')}.`, where.child(['stability']));
      if (typeof event.since !== 'string' || !/^\d+\.\d+\.\d+/.test(event.since)) report('EVT-2', 'x-orcon-event.since is the Orcon version that introduced the event (1.0.0).', where.child(['since']));
    }
    const body = model.requestBody(entry);
    const media = body ? model.mediaTypes(body.node, body.loc).find((m) => isJsonMedia(m.type)) : undefined;
    if (!media || typeof media.media.schema?.$ref !== 'string') {
      report('EVT-2', 'The event payload is a named schema ($ref).', body ? body.loc : entry.loc);
      continue;
    }
    const schemaLoc = media.loc.child(['schema']);
    const schema = model.deref(media.media.schema, schemaLoc);
    if (isObject(schema.node)) {
      const hasDescription = typeof schema.node.description === 'string' && schema.node.description.trim() !== '';
      if (!hasDescription) report('EVT-2', 'The event payload schema has a description.', schema.loc);
    }
    const hasExample = media.media.example !== undefined || (isObject(media.media.examples) && Object.keys(media.media.examples).length > 0);
    if (!hasExample) report('EVT-2', 'The event payload has an example.', media.loc);
    starts.push({ schema: media.media.schema, loc: schemaLoc });
  }
  model.walkSchemas(starts, (schema, loc) => {
    if (!isObject(schema.properties)) return;
    for (const name of Object.keys(schema.properties)) {
      if (!EXECUTION_DATA.test(name)) continue;
      const prop = model.deref(schema.properties[name], loc.child(['properties', name]));
      const marked = schema.properties[name]?.['x-orcon-data'] === 'protected' || prop.node?.['x-orcon-data'] === 'protected';
      if (!marked) report('FIELD-8', `Execution data field "${name}" carries x-orcon-data: protected.`, prop.loc);
    }
  });
});

/**
 * Constructs the Go generator cannot represent (VER-9). Measured with the generator pinned
 * in tools/go.mod (oapi-codegen v2.8.0, std-http-server + strict-server + models +
 * embedded-spec + nullable types) on 2026-10-13:
 *
 * - hard failure: a `type` array other than [T, "null"] ("multi-type"); a $ref that points
 *   inside a schema's $defs (the bundle step hoists those, so only unbundled input fails);
 * - silently wrong Go: prefixItems (-> []interface{}), $dynamicRef (-> interface{}),
 *   patternProperties (-> map[string]interface{}), not (-> interface{}), and the keywords
 *   if/then/else, dependentSchemas and unevaluatedProperties, which are dropped without a
 *   warning so the Go type accepts what the contract forbids;
 * - supported or harmless: allOf, anyOf, oneOf with a discriminator, const, contains,
 *   propertyNames, [T, "null"] (nullable.Nullable[T]), contentEncoding, readOnly/writeOnly
 *   (one Go struct for both directions), examples, exclusiveMinimum, multipleOf, formats.
 *
 * oneOf without a discriminator does generate (an untyped union over json.RawMessage with
 * numbered members), but it loses the names the SDKs need, so it stays on the list.
 */
const FORBIDDEN_KEYWORDS = ['prefixItems', '$dynamicRef', '$dynamicAnchor', 'if', 'then', 'else', 'dependentSchemas', 'unevaluatedProperties', 'patternProperties', 'not'];

export const codegenSubset = defineRule((model, report) => {
  for (const { schema, loc } of model.allSchemas) {
    for (const keyword of FORBIDDEN_KEYWORDS) {
      if (keyword in schema) {
        report('VER-9', `"${keyword}" is not supported by the Go generator; rewrite the schema without it.`, loc.child([keyword]));
      }
    }
    if (Array.isArray(schema.type)) {
      const nonNull = schema.type.filter((t) => t !== 'null');
      if (schema.type.length > 2 || nonNull.length !== 1) {
        report('VER-9', 'A type array is allowed only as [T, "null"].', loc.child(['type']));
      }
    }
    if (Array.isArray(schema.oneOf) && !isObject(schema.discriminator)) {
      report('VER-9', 'oneOf needs a discriminator.', loc.child(['oneOf']));
    }
  }
});
