// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0
//
// Expression runtime (ADR-0018 section 1). It defines __orcon_run, the entry
// point the sandbox calls for every evaluation, and the n8n-compatible names
// an expression sees. Behaviour follows the public n8n documentation and
// black-box observation; no n8n source is used.
//
// Protocol with the host (internal/expr/session.go):
//   request  JSON {e, m, j, b, i, r, n, z, x, w, v, p, q, l}
//     e source, m mode ("v" value, "s" string), j $json, b $binary,
//     i $itemIndex, r $runIndex, n activation time (ms), z time zone,
//     x $execution, w $workflow, v $vars, p $prevNode, q $parameter,
//     l lenient mode (corpus dry runs)
//   response JSON {v: value} | {u: 1} (undefined) | {s: string} | {x: {n, m}}
(function () {
  'use strict';
  const internal = globalThis.__orcon;
  const host = internal.host;
  const { DateTime, Duration, Interval, Settings } = globalThis.luxon;

  const OP_INPUT_ALL = 1;
  const OP_NODE_META = 2;
  const OP_NODE_ITEMS = 3;
  const OP_NODE_PAIRED = 4;
  const OP_RANDOM = 5;
  const OP_RANDOM_BYTES = 6;

  const jsonParse = JSON.parse;
  const jsonStringify = JSON.stringify;
  const NativeFunction = Function;

  Settings.defaultLocale = 'en-US';
  Settings.throwOnInvalid = false;

  class ExpressionError extends Error {
    constructor(message) {
      super(message);
      // Assigning this.name would fail: the prelude freezes Error.prototype,
      // and a non-writable inherited property blocks assignment.
      Object.defineProperty(this, 'name', { value: 'ExpressionError', configurable: true });
    }
  }

  function hostJSON(op, payload) {
    const r = host(op, payload);
    return r === undefined ? undefined : jsonParse(r);
  }

  // Deterministic randomness: the host draws from the activation's stream.
  Math.random = function random() { return Number(host(OP_RANDOM, '')); };
  const cryptoObject = {
    randomUUID() {
      const hex = host(OP_RANDOM_BYTES, '16');
      const b = [];
      for (let i = 0; i < 16; i++) b.push(parseInt(hex.substr(i * 2, 2), 16));
      b[6] = (b[6] & 0x0f) | 0x40;
      b[8] = (b[8] & 0x3f) | 0x80;
      const h = b.map((x) => x.toString(16).padStart(2, '0')).join('');
      return h.slice(0, 8) + '-' + h.slice(8, 12) + '-' + h.slice(12, 16) + '-' + h.slice(16, 20) + '-' + h.slice(20);
    },
    getRandomValues(array) {
      const hex = host(OP_RANDOM_BYTES, String(array.byteLength));
      const view = new Uint8Array(array.buffer, array.byteOffset, array.byteLength);
      for (let i = 0; i < view.length; i++) view[i] = parseInt(hex.substr(i * 2, 2), 16);
      return array;
    },
  };
  Object.defineProperty(globalThis, 'crypto', { value: cryptoObject, writable: true, configurable: true });

  // ---- helpers -----------------------------------------------------------

  function isEmptyValue(v) {
    if (v === undefined || v === null || v === '') return true;
    if (Array.isArray(v)) return v.length === 0;
    if (typeof v === 'object') return Object.keys(v).length === 0;
    return false;
  }

  function lenientProxy() {
    const target = function () {};
    const handler = {
      get(_, key) {
        if (key === Symbol.toPrimitive) return () => '';
        if (key === 'toString' || key === 'valueOf' || key === 'toJSON') return () => '';
        if (key === 'then') return undefined;
        if (key === Symbol.iterator) return function* () {};
        if (key === 'length') return 0;
        return proxy;
      },
      apply() { return proxy; },
      construct() { return proxy; },
      has() { return true; },
    };
    const proxy = new Proxy(target, handler);
    return proxy;
  }

  // ---- evaluation context ------------------------------------------------

  // Every name of the context. An expression is compiled with only the names
  // its text mentions, and each is built per call only when used, so a cheap
  // expression does not pay for Luxon objects or proxies it never reads.
  const NAMES = [
    '$', '$json', '$binary', '$input', '$node', '$now', '$today', '$execution', '$workflow',
    '$runIndex', '$itemIndex', '$position', '$prevNode', '$vars', '$env', '$parameter',
    '$jmespath', '$ifEmpty', '$if', '$min', '$max', '$items',
    'DateTime', 'Duration', 'Interval',
  ];

  function wrapItem(raw, index) {
    return { json: raw.json, binary: raw.binary === undefined ? {} : raw.binary, pairedItem: { item: index } };
  }

  let currentZone;
  function useZone(zone) {
    if (zone !== currentZone) {
      Settings.defaultZone = zone;
      currentZone = zone;
    }
  }

  const ifEmpty = (value, fallback) => (isEmptyValue(value) ? fallback : value);
  const ifFn = (cond, a, b) => (cond ? a : b);
  const minFn = (...n) => Math.min(...n);
  const maxFn = (...n) => Math.max(...n);
  const jmespath = () => { throw new ExpressionError('$jmespath is not available'); };
  const envDenied = new Proxy({}, {
    get() { throw new ExpressionError('access to env vars denied'); },
    has() { throw new ExpressionError('access to env vars denied'); },
    set() { return false; },
    defineProperty() { return false; },
  });
  // The helpers below are shared by every evaluation of the runtime; freezing
  // them keeps an expression from leaving state on them.
  function deepFreeze(v) {
    if (v !== null && typeof v === 'object' && !Object.isFrozen(v)) {
      Object.freeze(v);
      for (const k of Object.keys(v)) deepFreeze(v[k]);
    }
    return v;
  }

  Object.freeze(ifEmpty);
  Object.freeze(ifFn);
  Object.freeze(minFn);
  Object.freeze(maxFn);
  Object.freeze(jmespath);

  // st holds what one call builds lazily and shares between names.
  function nodeHelpers(req, st) {
    if (st.node) return st.node;
    const nodeItems = (name) => (hostJSON(OP_NODE_ITEMS, name) || []).map((it, i) => wrapItem(it, i));
    const metaOf = (name) => {
      if (typeof name !== 'string') throw new ExpressionError('node name must be a string');
      const meta = hostJSON(OP_NODE_META, name);
      if (!meta || !meta.exists) throw new ExpressionError("Referenced node doesn't exist");
      return meta;
    };
    const pairedItem = (name, meta) => {
      if (!meta.executed) throw new ExpressionError('Referenced node is unexecuted');
      const it = hostJSON(OP_NODE_PAIRED, jsonStringify({ n: name, i: req.i }));
      if (it === undefined) throw new ExpressionError("Can't get data for expression: no item of '" + name + "' is paired with the current item");
      return wrapItem(it, req.i);
    };
    st.node = { nodeItems, metaOf, pairedItem };
    return st.node;
  }

  function inputItems(st) {
    if (st.all === undefined) st.all = (hostJSON(OP_INPUT_ALL, '') || []).map((it, i) => wrapItem(it, i));
    return st.all;
  }

  // The activation time is constant, so the DateTime values built from it are
  // too: every evaluation of an activation shares them (DateTime is immutable).
  let clockKey = '';
  let clockValue;
  function clock(req) {
    const zone = req.z || 'UTC';
    const key = req.n + '|' + zone;
    if (key !== clockKey) {
      const now = DateTime.fromMillis(req.n, { zone });
      clockValue = { now, get today() { const t = now.startOf('day'); Object.defineProperty(this, 'today', { value: t }); return t; } };
      clockKey = key;
    }
    return clockValue;
  }

  // State holds the sections of one request, parsed on first use: most
  // expressions never touch most of them, and parsing JSON in the sandbox is
  // the largest cost of a call.
  class State {
    constructor(parts) {
      this.parts = parts;
      this.cache = [];
    }

    section(index, fallback) {
      if (this.cache[index] === undefined) {
        const text = this.parts[index];
        this.cache[index] = text === undefined || text === '' ? fallback : jsonParse(text);
      }
      return this.cache[index];
    }

    json() { return this.section(1, {}); }
    binary() { return this.section(2, {}); }
    vars() { return this.section(3, {}); }
    params() { return this.section(4, {}); }
  }

  const MAKERS = {
    $(req, st) {
      const h = nodeHelpers(req, st);
      return (name) => {
        const meta = h.metaOf(name);
        const items = () => {
          if (!meta.executed) throw new ExpressionError('Referenced node is unexecuted');
          return h.nodeItems(name);
        };
        return {
          get isExecuted() { return meta.executed; },
          get params() { return meta.params === undefined ? {} : meta.params; },
          get context() { return {}; },
          get item() { return h.pairedItem(name, meta); },
          first() { return items()[0]; },
          last() { const a = items(); return a[a.length - 1]; },
          all() { return items(); },
          itemMatching(i) { return items()[i]; },
        };
      };
    },
    $json: (req, st) => st.json(),
    $binary: (req, st) => st.binary(),
    $input(req, st) {
      return {
        get item() { return wrapItem({ json: st.json(), binary: st.binary() }, req.i); },
        all() { return inputItems(st); },
        first() { return inputItems(st)[0]; },
        last() { const a = inputItems(st); return a[a.length - 1]; },
        itemMatching(i) { return inputItems(st)[i]; },
        params: st.params(),
        context: {},
      };
    },
    $node(req, st) {
      const h = nodeHelpers(req, st);
      return new Proxy({}, {
        get(_, name) {
          if (typeof name !== 'string') return undefined;
          const meta = h.metaOf(name);
          const it = h.pairedItem(name, meta);
          return { json: it.json, data: it.json, binary: it.binary, parameter: meta.params === undefined ? {} : meta.params, context: {}, runIndex: 0 };
        },
        has(_, name) { return typeof name === 'string' && !!(hostJSON(OP_NODE_META, name) || {}).exists; },
      });
    },
    $now: (req) => clock(req).now,
    $today: (req) => clock(req).today,
    $execution(req) {
      // The variables are read-only and constant during the activation.
      const x = Object.assign({ id: '', mode: 'production', resumeUrl: '', resumeFormUrl: '', variables: {} }, req.x);
      deepFreeze(x.variables);
      return x;
    },
    $workflow: (req) => Object.assign({ id: '', name: '', active: false }, req.w),
    $runIndex: (req) => req.r,
    $itemIndex: (req) => req.i,
    $position: (req) => req.i,
    $prevNode: (req) => Object.assign({ name: '', outputIndex: 0, runIndex: 0 }, req.p),
    $vars: (req, st) => st.vars(),
    $env: () => envDenied,
    $parameter: (req, st) => st.params(),
    $jmespath: () => internal.jmespath || jmespath,
    $ifEmpty: () => ifEmpty,
    $if: () => ifFn,
    $min: () => minFn,
    $max: () => maxFn,
    $items(req, st) {
      const h = nodeHelpers(req, st);
      return (name) => {
        if (name === undefined) return inputItems(st);
        const meta = h.metaOf(name);
        return meta.executed ? h.nodeItems(name) : [];
      };
    },
    DateTime: () => DateTime,
    Duration: () => Duration,
    Interval: () => Interval,
  };

  const lenient = lenientProxy();
  const LENIENT = {
    $: () => () => lenient, $json: () => lenient, $binary: () => lenient, $input: () => lenient,
    $node: () => lenient, $vars: () => lenient, $parameter: () => lenient, $items: () => () => lenient,
  };

  // ---- compile and run ---------------------------------------------------

  const MAX_CACHED = 256;
  let cache = new Map();

  function compile(source) {
    let entry = cache.get(source);
    if (entry === undefined) {
      const dynamic = /\beval\b|\bFunction\b|\barguments\b|\bwith\b/.test(source);
      const used = NAMES.filter((n) => dynamic || n === '$' || source.includes(n));
      entry = { fn: new NativeFunction(...used, 'return (\n' + source + '\n);'), used };
      if (cache.size >= MAX_CACHED) cache = new Map();
      cache.set(source, entry);
    }
    return entry;
  }

  function run(req, parts, entry) {
    useZone(req.z || 'UTC');
    const st = new State(parts);
    const makers = req.l ? Object.assign({}, MAKERS, LENIENT) : MAKERS;
    const args = new Array(entry.used.length);
    for (let i = 0; i < args.length; i++) args[i] = makers[entry.used[i]](req, st);
    return entry.fn(...args);
  }

  // failure reports a thrown value. compiling marks a syntax error found
  // while compiling the expression, as opposed to one thrown while it ran.
  function failure(e, compiling) {
    let name = 'Error';
    let message;
    if (e !== null && typeof e === 'object') {
      name = String(e.name === undefined ? 'Error' : e.name);
      message = String(e.message === undefined ? '' : e.message);
    } else {
      message = String(e);
    }
    const body = { n: name, m: message };
    if (compiling) body.c = 1;
    return '{"x":' + jsonStringify(body) + '}';
  }

  function serialize(value) {
    if (value === undefined) return '{"u":1}';
    const text = jsonStringify(value);
    if (text === undefined) return '{"u":1}';
    return '{"v":' + text + '}';
  }

  globalThis.__orcon_run = function (requestText) {
    let req;
    let entry;
    const parts = requestText.split('\u0001');
    try {
      req = jsonParse(parts[0]);
      entry = compile(req.e);
    } catch (e) {
      return failure(e, true);
    }
    try {
      const value = run(req, parts, entry);
      if (req.m === 's') return '{"s":' + jsonStringify(String(value)) + '}';
      return serialize(value);
    } catch (e) {
      return failure(e, false);
    }
  };

  internal.ExpressionError = ExpressionError;
  internal.isEmptyValue = isEmptyValue;
})();
