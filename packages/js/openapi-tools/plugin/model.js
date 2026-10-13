// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * A resolved, location-aware view of an OpenAPI document that every `orcon/*`
 * rule shares. Rules run as `Root` visitors and ask the model for operations,
 * parameters, responses and schemas, so a `$ref` into another file is followed
 * the same way everywhere and every finding points at the node that is wrong.
 */

export const METHODS = ['get', 'put', 'post', 'delete', 'options', 'head', 'patch', 'trace'];
export const MUTATING = ['post', 'put', 'patch', 'delete'];

const isObject = (v) => v !== null && typeof v === 'object' && !Array.isArray(v);
export { isObject };

const SCHEMA_MAP_KEYS = ['properties', 'patternProperties', '$defs', 'definitions', 'dependentSchemas'];
const SCHEMA_LIST_KEYS = ['allOf', 'anyOf', 'oneOf', 'prefixItems'];
const SCHEMA_SINGLE_KEYS = [
  'items',
  'additionalProperties',
  'not',
  'if',
  'then',
  'else',
  'contains',
  'propertyNames',
  'unevaluatedProperties',
  'unevaluatedItems',
];

const cache = new WeakMap();

/** Returns the (cached) model of the document being linted. */
export function getModel(root, ctx) {
  let model = cache.get(root);
  if (!model) {
    model = new Model(root, ctx);
    cache.set(root, model);
  }
  return model;
}

class Model {
  constructor(root, ctx) {
    this.root = root;
    this.ctx = ctx;
    this.rootLoc = ctx.location;
    this._operations = undefined;
    this._schemas = undefined;
  }

  /** Follows a `$ref` chain. Returns { node, loc, raw } where raw is the node as written. */
  deref(node, loc) {
    let cur = node;
    let curLoc = loc;
    for (let i = 0; i < 30 && isObject(cur) && typeof cur.$ref === 'string'; i += 1) {
      const resolved = this.ctx.resolve(cur, curLoc.source.absoluteRef);
      if (!resolved || resolved.node === undefined) return { node: undefined, loc: curLoc, raw: node };
      cur = resolved.node;
      curLoc = resolved.location;
    }
    return { node: cur, loc: curLoc, raw: node };
  }

  get components() {
    const c = this.deref(this.root.components, this.rootLoc.child(['components']));
    return isObject(c.node) ? c : { node: {}, loc: c.loc };
  }

  /** components.schemas.<name> resolved, or undefined. */
  namedSchema(name) {
    const schemas = this.components.node.schemas;
    if (!isObject(schemas) || !(name in schemas)) return undefined;
    const loc = this.components.loc.child(['schemas', name]);
    return this.deref(schemas[name], loc);
  }

  /** Every operation of `paths` and `webhooks`, with its resolved path item. */
  get operations() {
    if (this._operations) return this._operations;
    const out = [];
    const collect = (kind, mapKey) => {
      const map = this.root[mapKey];
      if (!isObject(map)) return;
      const mapLoc = this.rootLoc.child([mapKey]);
      for (const key of Object.keys(map)) {
        if (key.startsWith('x-')) continue;
        const item = this.deref(map[key], mapLoc.child([key]));
        if (!isObject(item.node)) continue;
        for (const method of METHODS) {
          const op = item.node[method];
          if (!isObject(op)) continue;
          const tags = Array.isArray(op.tags) ? op.tags : [];
          out.push({
            kind,
            key,
            method,
            op,
            loc: item.loc.child([method]),
            item: item.node,
            itemLoc: item.loc,
            protocol: tags.includes('protocol'),
          });
        }
      }
    };
    collect('path', 'paths');
    collect('webhook', 'webhooks');
    this._operations = out;
    return out;
  }

  get pathOperations() {
    return this.operations.filter((o) => o.kind === 'path');
  }

  /** Path operations the grammar rules apply to (everything but protocol-tagged ones). */
  get contractOperations() {
    return this.operations.filter((o) => o.kind === 'path' && !o.protocol);
  }

  /** Resolved parameters of an operation (path-item level merged with operation level). */
  parameters(entry) {
    const merged = new Map();
    const add = (list, listLoc) => {
      if (!Array.isArray(list)) return;
      list.forEach((raw, i) => {
        const r = this.deref(raw, listLoc.child([i]));
        if (!isObject(r.node)) return;
        merged.set(`${r.node.in}:${r.node.name}`, { ...this.paramEntry(r), raw });
      });
    };
    add(entry.item.parameters, entry.itemLoc.child(['parameters']));
    add(entry.op.parameters, entry.loc.child(['parameters']));
    return [...merged.values()];
  }

  paramEntry(r) {
    const p = r.node;
    let schema = p.schema;
    let schemaLoc = r.loc.child(['schema']);
    if (!schema && isObject(p.content)) {
      const mt = Object.keys(p.content)[0];
      schema = p.content[mt]?.schema;
      schemaLoc = r.loc.child(['content', mt, 'schema']);
    }
    return { name: p.name, in: p.in, param: p, loc: r.loc, schema, schemaLoc };
  }

  /** Resolved responses of an operation: [{ status, resp, loc }]. */
  responses(entry) {
    const out = [];
    const responses = entry.op.responses;
    if (!isObject(responses)) return out;
    for (const status of Object.keys(responses)) {
      if (status.startsWith('x-')) continue;
      const r = this.deref(responses[status], entry.loc.child(['responses', status]));
      if (isObject(r.node)) out.push({ status, resp: r.node, loc: r.loc });
    }
    return out;
  }

  /** Resolved media types of a request body or response: [{ type, media, loc }]. */
  mediaTypes(container, containerLoc) {
    const out = [];
    if (!isObject(container?.content)) return out;
    for (const type of Object.keys(container.content)) {
      const media = container.content[type];
      if (isObject(media)) out.push({ type, media, loc: containerLoc.child(['content', type]) });
    }
    return out;
  }

  /** The resolved request body of an operation, or undefined. */
  requestBody(entry) {
    if (entry.op.requestBody === undefined) return undefined;
    const r = this.deref(entry.op.requestBody, entry.loc.child(['requestBody']));
    return isObject(r.node) ? r : undefined;
  }

  /** Resolved headers of a response: [{ name, header, loc }]. */
  responseHeaders(resp, respLoc) {
    const out = [];
    if (!isObject(resp.headers)) return out;
    for (const name of Object.keys(resp.headers)) {
      const r = this.deref(resp.headers[name], respLoc.child(['headers', name]));
      if (isObject(r.node)) out.push({ name, header: r.node, loc: r.loc });
    }
    return out;
  }

  /**
   * Walks every schema reachable from `starts` ([{ schema, loc }]) once, following
   * `$ref`. `visit(schema, loc, raw)` receives the resolved schema node.
   * `seen` lets callers share de-duplication across several walks.
   */
  walkSchemas(starts, visit, seen = new Set()) {
    const stack = [...starts].reverse();
    while (stack.length > 0) {
      const { schema, loc } = stack.pop();
      const r = this.deref(schema, loc);
      if (!isObject(r.node) || seen.has(r.node)) continue;
      seen.add(r.node);
      visit(r.node, r.loc, r.raw);
      const children = [];
      for (const key of SCHEMA_MAP_KEYS) {
        if (isObject(r.node[key])) {
          for (const name of Object.keys(r.node[key])) children.push({ schema: r.node[key][name], loc: r.loc.child([key, name]) });
        }
      }
      for (const key of SCHEMA_LIST_KEYS) {
        if (Array.isArray(r.node[key])) r.node[key].forEach((s, i) => children.push({ schema: s, loc: r.loc.child([key, i]) }));
      }
      for (const key of SCHEMA_SINGLE_KEYS) {
        if (isObject(r.node[key])) children.push({ schema: r.node[key], loc: r.loc.child([key]) });
        else if (Array.isArray(r.node[key]) && key === 'items') {
          r.node[key].forEach((s, i) => children.push({ schema: s, loc: r.loc.child([key, i]) }));
        }
      }
      for (let i = children.length - 1; i >= 0; i -= 1) stack.push(children[i]);
    }
  }

  /** Schemas an operation uses: parameters, request body, responses and response headers. */
  operationSchemaRoots(entries) {
    const roots = [];
    for (const entry of entries) {
      for (const p of this.parameters(entry)) if (p.schema) roots.push({ schema: p.schema, loc: p.schemaLoc });
      const body = this.requestBody(entry);
      if (body) for (const mt of this.mediaTypes(body.node, body.loc)) if (mt.media.schema) roots.push({ schema: mt.media.schema, loc: mt.loc.child(['schema']) });
      for (const resp of this.responses(entry)) {
        for (const mt of this.mediaTypes(resp.resp, resp.loc)) if (mt.media.schema) roots.push({ schema: mt.media.schema, loc: mt.loc.child(['schema']) });
        for (const h of this.responseHeaders(resp.resp, resp.loc)) if (h.header.schema) roots.push({ schema: h.header.schema, loc: h.loc.child(['schema']) });
      }
    }
    return roots;
  }

  /** Entry points: every schema a document defines or an operation uses. */
  schemaRoots() {
    const roots = [];
    const schemas = this.components.node.schemas;
    if (isObject(schemas)) {
      for (const name of Object.keys(schemas)) roots.push({ schema: schemas[name], loc: this.components.loc.child(['schemas', name]) });
    }
    roots.push(...this.operationSchemaRoots(this.operations));
    return roots;
  }

  /**
   * Schemas reachable only from protocol-tagged operations. Protocol endpoints follow an
   * external standard (RFC 6749 field names, for example), so the field rules skip them.
   */
  protocolOnlySchemas() {
    const fromProtocol = new Set();
    this.walkSchemas(this.operationSchemaRoots(this.operations.filter((o) => o.protocol)), () => {}, fromProtocol);
    const fromOthers = new Set();
    this.walkSchemas(this.operationSchemaRoots(this.operations.filter((o) => !o.protocol)), () => {}, fromOthers);
    return new Set([...fromProtocol].filter((node) => !fromOthers.has(node)));
  }

  /** Every unique schema of the document, as [{ schema, loc }] (resolved nodes). */
  get allSchemas() {
    if (this._schemas) return this._schemas;
    const out = [];
    this.walkSchemas(this.schemaRoots(), (schema, loc) => out.push({ schema, loc }), this.protocolOnlySchemas());
    this._schemas = out;
    return out;
  }

  /** Schemas reachable from the request bodies of path operations. */
  requestSchemas() {
    const out = [];
    const starts = [];
    for (const entry of this.contractOperations) {
      const body = this.requestBody(entry);
      if (!body) continue;
      for (const mt of this.mediaTypes(body.node, body.loc)) {
        if (isJsonMedia(mt.type) && mt.media.schema) starts.push({ schema: mt.media.schema, loc: mt.loc.child(['schema']) });
      }
    }
    this.walkSchemas(starts, (schema, loc) => out.push({ schema, loc }));
    return out;
  }
}

export function isJsonMedia(type) {
  return /(^|\/|\+)json(;|$)/i.test(type);
}

/** The types of a schema as an array, ignoring "null". */
export function typesOf(schema) {
  if (!isObject(schema)) return [];
  const t = schema.type;
  const list = Array.isArray(t) ? t : t === undefined ? [] : [t];
  return list.filter((x) => x !== 'null');
}

export function isObjectSchema(schema) {
  return typesOf(schema).includes('object') || isObject(schema?.properties);
}

/** Properties of a schema including those merged through allOf (shallow, unresolved members are skipped). */
export function mergedProperties(model, schema, loc, seen = new Set()) {
  const out = [];
  if (!isObject(schema) || seen.has(schema)) return out;
  seen.add(schema);
  if (isObject(schema.properties)) {
    for (const name of Object.keys(schema.properties)) out.push({ name, schema: schema.properties[name], loc: loc.child(['properties', name]) });
  }
  if (Array.isArray(schema.allOf)) {
    schema.allOf.forEach((member, i) => {
      const r = model.deref(member, loc.child(['allOf', i]));
      out.push(...mergedProperties(model, r.node, r.loc, seen));
    });
  }
  return out;
}

/** Last name of a `$ref` string ("#/components/schemas/Flow" -> "Flow"). */
export function refName(raw) {
  if (!isObject(raw) || typeof raw.$ref !== 'string') return undefined;
  const parts = raw.$ref.split('/');
  return parts[parts.length - 1];
}

export function splitPath(path) {
  return path.split('/').slice(1);
}
