// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0
//
// $jmespath(data, query): a JMESPath evaluator (https://jmespath.org/specification.html)
// written from the language specification. It supports the full grammar:
// identifiers and quoted identifiers, sub-expressions, indexes, slices,
// projections, flattening, filters, pipes, multiselect lists and hashes,
// literals, comparators, boolean operators, expression references and the
// built-in functions of the specification.
(function () {
  'use strict';
  const internal = globalThis.__orcon;

  class JMESPathError extends Error {
    constructor(message) {
      super(message);
      Object.defineProperty(this, 'name', { value: 'JMESPathError', configurable: true });
    }
  }
  const fail = (m) => { throw new JMESPathError(m); };

  // ---- tokenizer ----------------------------------------------------------

  const isIdentStart = (c) => (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c === '_';
  const isDigit = (c) => c >= '0' && c <= '9';
  const isIdent = (c) => isIdentStart(c) || isDigit(c);

  function tokenize(src) {
    const out = [];
    let i = 0;
    const n = src.length;
    const push = (type, value, pos) => out.push({ type, value, pos });
    while (i < n) {
      const c = src[i];
      if (c === ' ' || c === '\t' || c === '\n' || c === '\r') { i++; continue; }
      const start = i;
      if (isIdentStart(c)) {
        while (i < n && isIdent(src[i])) i++;
        push('ident', src.slice(start, i), start);
      } else if (isDigit(c) || (c === '-' && isDigit(src[i + 1] || ''))) {
        i++;
        while (i < n && isDigit(src[i])) i++;
        push('number', parseInt(src.slice(start, i), 10), start);
      } else if (c === '"') {
        i++;
        while (i < n && src[i] !== '"') i += src[i] === '\\' ? 2 : 1;
        if (i >= n) fail('Unterminated quoted identifier at ' + start);
        i++;
        let text;
        try { text = JSON.parse(src.slice(start, i)); } catch (e) { fail('Invalid quoted identifier at ' + start); }
        push('quoted', text, start);
      } else if (c === "'") {
        i++;
        let text = '';
        while (i < n && src[i] !== "'") {
          if (src[i] === '\\' && (src[i + 1] === "'" || src[i + 1] === '\\')) { text += src[i + 1]; i += 2; } else { text += src[i]; i++; }
        }
        if (i >= n) fail('Unterminated raw string at ' + start);
        i++;
        push('literal', text, start);
      } else if (c === '`') {
        i++;
        let text = '';
        while (i < n && src[i] !== '`') {
          if (src[i] === '\\' && src[i + 1] === '`') { text += '`'; i += 2; } else { text += src[i]; i++; }
        }
        if (i >= n) fail('Unterminated literal at ' + start);
        i++;
        let value;
        try { value = JSON.parse(text.trim()); } catch (e) {
          try { value = JSON.parse('"' + text.trim() + '"'); } catch (e2) { fail('Invalid JSON literal at ' + start); }
        }
        push('literal', value, start);
      } else if (c === '[') {
        if (src[i + 1] === ']') { i += 2; push('flatten', '[]', start); } else if (src[i + 1] === '?') { i += 2; push('filter', '[?', start); } else { i++; push('lbracket', '[', start); }
      } else if (c === '|') {
        if (src[i + 1] === '|') { i += 2; push('or', '||', start); } else { i++; push('pipe', '|', start); }
      } else if (c === '&') {
        if (src[i + 1] === '&') { i += 2; push('and', '&&', start); } else { i++; push('expref', '&', start); }
      } else if (c === '<') {
        if (src[i + 1] === '=') { i += 2; push('lte', '<=', start); } else { i++; push('lt', '<', start); }
      } else if (c === '>') {
        if (src[i + 1] === '=') { i += 2; push('gte', '>=', start); } else { i++; push('gt', '>', start); }
      } else if (c === '=') {
        if (src[i + 1] === '=') { i += 2; push('eq', '==', start); } else fail('Unexpected "=" at ' + start);
      } else if (c === '!') {
        if (src[i + 1] === '=') { i += 2; push('ne', '!=', start); } else { i++; push('not', '!', start); }
      } else if ('.*,:{}()]@'.includes(c)) {
        i++;
        push({ '.': 'dot', '*': 'star', ',': 'comma', ':': 'colon', '{': 'lbrace', '}': 'rbrace', '(': 'lparen', ')': 'rparen', ']': 'rbracket', '@': 'current' }[c], c, start);
      } else {
        fail('Unexpected character "' + c + '" at ' + start);
      }
    }
    push('eof', null, n);
    return out;
  }

  // ---- parser (Pratt) -----------------------------------------------------

  const BP = {
    eof: 0, ident: 0, quoted: 0, literal: 0, rbracket: 0, rparen: 0, comma: 0, rbrace: 0, number: 0, current: 0, expref: 0, colon: 0,
    pipe: 1, or: 2, and: 3, eq: 5, gt: 5, lt: 5, gte: 5, lte: 5, ne: 5, flatten: 9, star: 20, filter: 21, dot: 40, not: 45, lbrace: 50, lbracket: 55, lparen: 60,
  };
  const PROJECTION_STOP = 10;

  function parse(src) {
    const tokens = tokenize(src);
    let p = 0;
    const peek = () => tokens[p];
    const next = () => tokens[p++];
    const expect = (type) => {
      const t = next();
      if (t.type !== type) fail('Expected ' + type + ' but found ' + t.type + ' at ' + t.pos);
      return t;
    };

    function expression(rbp) {
      let t = next();
      let left = nud(t);
      while (rbp < BP[peek().type]) {
        t = next();
        left = led(t, left);
      }
      return left;
    }

    function nud(t) {
      switch (t.type) {
        case 'literal': return { t: 'literal', v: t.value };
        case 'ident': return { t: 'field', name: t.value };
        case 'quoted':
          if (peek().type === 'lparen') fail('Quoted identifier cannot be a function name at ' + t.pos);
          return { t: 'field', name: t.value };
        case 'current': return { t: 'current' };
        case 'expref': return { t: 'expref', child: expression(BP.expref) };
        case 'not': return { t: 'not', child: expression(BP.not) };
        case 'lparen': { const e = expression(0); expect('rparen'); return e; }
        case 'star': return { t: 'valueProjection', left: { t: 'current' }, right: projectionRight(BP.star) };
        case 'flatten': return { t: 'projection', left: { t: 'flatten', child: { t: 'current' } }, right: projectionRight(BP.flatten) };
        case 'filter': return filterProjection({ t: 'current' });
        case 'lbrace': return multiHash();
        case 'lbracket': {
          const k = peek().type;
          if (k === 'number' || k === 'colon') {
            const right = indexExpression();
            if (right.t === 'slice') return { t: 'projection', left: right, right: projectionRight(BP.star) };
            return right;
          }
          if (k === 'star' && tokens[p + 1].type === 'rbracket') {
            next(); next();
            return { t: 'projection', left: { t: 'current' }, right: projectionRight(BP.star) };
          }
          return multiList();
        }
        default: return fail('Unexpected token ' + t.type + ' at ' + t.pos);
      }
    }

    function led(t, left) {
      switch (t.type) {
        case 'dot': {
          if (peek().type === 'star') { next(); return { t: 'valueProjection', left, right: projectionRight(BP.star) }; }
          return { t: 'subexpression', left, right: dotRight() };
        }
        case 'pipe': return { t: 'pipe', left, right: expression(BP.pipe) };
        case 'or': return { t: 'or', left, right: expression(BP.or) };
        case 'and': return { t: 'and', left, right: expression(BP.and) };
        case 'lparen': {
          if (left.t !== 'field') fail('Invalid function call at ' + t.pos);
          const args = [];
          while (peek().type !== 'rparen') {
            args.push(expression(0));
            if (peek().type === 'comma') next();
          }
          next();
          return { t: 'function', name: left.name, args };
        }
        case 'filter': return filterProjection(left);
        case 'flatten': return { t: 'projection', left: { t: 'flatten', child: left }, right: projectionRight(BP.flatten) };
        case 'eq': case 'ne': case 'gt': case 'gte': case 'lt': case 'lte':
          return { t: 'comparator', op: t.type, left, right: expression(BP[t.type]) };
        case 'lbracket': {
          const k = peek().type;
          if (k === 'number' || k === 'colon') {
            const right = indexExpression();
            if (right.t === 'slice') return { t: 'projection', left, right: projectionRight(BP.star), slice: right };
            return { t: 'subexpression', left, right };
          }
          expect('star'); expect('rbracket');
          return { t: 'projection', left, right: projectionRight(BP.star) };
        }
        default: return fail('Unexpected token ' + t.type + ' at ' + t.pos);
      }
    }

    function indexExpression() {
      // After "[": an index, or a slice start.
      const parts = [null, null, null];
      let part = 0;
      let colons = 0;
      while (peek().type !== 'rbracket') {
        const t = next();
        if (t.type === 'colon') {
          colons++;
          part++;
          if (colons > 2) fail('Too many colons in slice at ' + t.pos);
        } else if (t.type === 'number') {
          parts[part] = t.value;
        } else {
          fail('Unexpected token in index ' + t.type + ' at ' + t.pos);
        }
      }
      next();
      if (colons === 0) return { t: 'index', value: parts[0] };
      return { t: 'slice', start: parts[0], stop: parts[1], step: parts[2] };
    }

    function projectionRight(bp) {
      const k = peek().type;
      if (BP[k] < PROJECTION_STOP) return { t: 'current' };
      if (k === 'dot') { next(); return dotRight(); }
      if (k === 'lbracket' || k === 'filter') return expression(bp);
      return fail('Unexpected token after projection: ' + k);
    }

    function dotRight() {
      const k = peek().type;
      if (k === 'ident' || k === 'quoted' || k === 'star') return expression(BP.dot);
      if (k === 'lbracket') { next(); return multiList(); }
      if (k === 'lbrace') { next(); return multiHash(); }
      return fail('Unexpected token after "." : ' + k);
    }

    function filterProjection(left) {
      const cond = expression(0);
      expect('rbracket');
      return { t: 'filterProjection', left, cond, right: projectionRight(BP.filter) };
    }

    function multiList() {
      const items = [];
      for (;;) {
        items.push(expression(0));
        const t = next();
        if (t.type === 'rbracket') break;
        if (t.type !== 'comma') fail('Expected "," or "]" at ' + t.pos);
      }
      return { t: 'multiList', items };
    }

    function multiHash() {
      const pairs = [];
      for (;;) {
        const k = next();
        if (k.type !== 'ident' && k.type !== 'quoted') fail('Expected an identifier as a key at ' + k.pos);
        expect('colon');
        pairs.push({ key: k.value, value: expression(0) });
        const t = next();
        if (t.type === 'rbrace') break;
        if (t.type !== 'comma') fail('Expected "," or "}" at ' + t.pos);
      }
      return { t: 'multiHash', pairs };
    }

    const ast = expression(0);
    if (peek().type !== 'eof') fail('Unexpected token ' + peek().type + ' at ' + peek().pos);
    return ast;
  }

  // ---- interpreter ----------------------------------------------------------

  const isObject = (v) => v !== null && typeof v === 'object' && !Array.isArray(v);
  const truthy = (v) => !(v === null || v === undefined || v === false || v === '' || (Array.isArray(v) && v.length === 0) || (isObject(v) && Object.keys(v).length === 0));
  const typeName = (v) => (v === null || v === undefined ? 'null' : Array.isArray(v) ? 'array' : typeof v);

  function deepEqual(a, b) {
    if (a === b) return true;
    if (Array.isArray(a)) return Array.isArray(b) && a.length === b.length && a.every((x, i) => deepEqual(x, b[i]));
    if (isObject(a)) {
      if (!isObject(b)) return false;
      const ka = Object.keys(a);
      return ka.length === Object.keys(b).length && ka.every((k) => Object.prototype.hasOwnProperty.call(b, k) && deepEqual(a[k], b[k]));
    }
    return false;
  }

  function sliceIndices(len, start, stop, step) {
    if (step === null) step = 1;
    if (step === 0) fail('Invalid slice, step cannot be 0');
    const clamp = (i, lo, hi) => (i < lo ? lo : i > hi ? hi : i);
    let s;
    let e;
    if (step > 0) {
      s = start === null ? 0 : clamp(start < 0 ? start + len : start, 0, len);
      e = stop === null ? len : clamp(stop < 0 ? stop + len : stop, 0, len);
    } else {
      s = start === null ? len - 1 : clamp(start < 0 ? start + len : start, -1, len - 1);
      e = stop === null ? -1 : clamp(stop < 0 ? stop + len : stop, -1, len - 1);
    }
    const out = [];
    if (step > 0) for (let i = s; i < e; i += step) out.push(i);
    else for (let i = s; i > e; i += step) out.push(i);
    return out;
  }

  function visit(node, v) {
    switch (node.t) {
      case 'literal': return node.v;
      case 'current': return v;
      case 'field': return isObject(v) && Object.prototype.hasOwnProperty.call(v, node.name) ? v[node.name] : null;
      case 'subexpression': return visit(node.right, visit(node.left, v));
      case 'pipe': return visit(node.right, visit(node.left, v));
      case 'index': {
        if (!Array.isArray(v)) return null;
        const i = node.value < 0 ? v.length + node.value : node.value;
        return i >= 0 && i < v.length ? v[i] : null;
      }
      case 'slice': {
        if (!Array.isArray(v)) return null;
        return sliceIndices(v.length, node.start, node.stop, node.step).map((i) => v[i]);
      }
      case 'projection': {
        const base = node.slice ? visit(node.slice, visit(node.left, v)) : visit(node.left, v);
        if (!Array.isArray(base)) return null;
        const out = [];
        for (const item of base) { const r = visit(node.right, item); if (r !== null) out.push(r); }
        return out;
      }
      case 'valueProjection': {
        const base = visit(node.left, v);
        if (!isObject(base)) return null;
        const out = [];
        for (const k of Object.keys(base)) { const r = visit(node.right, base[k]); if (r !== null) out.push(r); }
        return out;
      }
      case 'filterProjection': {
        const base = visit(node.left, v);
        if (!Array.isArray(base)) return null;
        const out = [];
        for (const item of base) {
          if (truthy(visit(node.cond, item))) { const r = visit(node.right, item); if (r !== null) out.push(r); }
        }
        return out;
      }
      case 'flatten': {
        const base = visit(node.child, v);
        if (!Array.isArray(base)) return null;
        const out = [];
        for (const item of base) { if (Array.isArray(item)) out.push(...item); else out.push(item); }
        return out;
      }
      case 'comparator': {
        const a = visit(node.left, v);
        const b = visit(node.right, v);
        if (node.op === 'eq') return deepEqual(a, b);
        if (node.op === 'ne') return !deepEqual(a, b);
        if (typeof a !== 'number' || typeof b !== 'number') return null;
        return node.op === 'gt' ? a > b : node.op === 'gte' ? a >= b : node.op === 'lt' ? a < b : a <= b;
      }
      case 'or': { const a = visit(node.left, v); return truthy(a) ? a : visit(node.right, v); }
      case 'and': { const a = visit(node.left, v); return truthy(a) ? visit(node.right, v) : a; }
      case 'not': return !truthy(visit(node.child, v));
      case 'multiList': return node.items.map((it) => visit(it, v));
      case 'multiHash': {
        if (v === null) return null;
        const out = {};
        for (const p of node.pairs) out[p.key] = visit(p.value, v);
        return out;
      }
      case 'expref': return { expref: node.child };
      case 'function': return callFunction(node.name, node.args.map((a) => visit(a, v)));
      default: return fail('Unknown node ' + node.t);
    }
  }

  function expect(name, value, ...types) {
    const t = typeName(value);
    if (!types.includes(t)) fail('Invalid type for ' + name + '(): expected ' + types.join(' or ') + ' but found ' + t);
  }

  const FUNCTIONS = {
    abs: (a) => { expect('abs', a, 'number'); return Math.abs(a); },
    avg: (a) => { expect('avg', a, 'array'); return a.length === 0 ? null : a.reduce((x, y) => x + y, 0) / a.length; },
    ceil: (a) => { expect('ceil', a, 'number'); return Math.ceil(a); },
    contains: (s, x) => { expect('contains', s, 'array', 'string'); return typeof s === 'string' ? (typeof x === 'string' && s.includes(x)) : s.some((i) => deepEqual(i, x)); },
    ends_with: (s, x) => { expect('ends_with', s, 'string'); expect('ends_with', x, 'string'); return s.endsWith(x); },
    floor: (a) => { expect('floor', a, 'number'); return Math.floor(a); },
    join: (g, a) => { expect('join', g, 'string'); expect('join', a, 'array'); for (const i of a) expect('join', i, 'string'); return a.join(g); },
    keys: (o) => { expect('keys', o, 'object'); return Object.keys(o); },
    length: (a) => { expect('length', a, 'string', 'array', 'object'); return typeof a === 'string' ? Array.from(a).length : Array.isArray(a) ? a.length : Object.keys(a).length; },
    map: (f, a) => { expectRef('map', f); expect('map', a, 'array'); return a.map((i) => visit(f.expref, i)); },
    max: (a) => { expect('max', a, 'array'); if (a.length === 0) return null; return extreme('max', a, (x, y) => x > y); },
    max_by: (a, f) => { expect('max_by', a, 'array'); expectRef('max_by', f); return extremeBy('max_by', a, f, (x, y) => x > y); },
    merge: (...os) => { const out = {}; for (const o of os) { expect('merge', o, 'object'); Object.assign(out, o); } return out; },
    min: (a) => { expect('min', a, 'array'); if (a.length === 0) return null; return extreme('min', a, (x, y) => x < y); },
    min_by: (a, f) => { expect('min_by', a, 'array'); expectRef('min_by', f); return extremeBy('min_by', a, f, (x, y) => x < y); },
    not_null: (...xs) => { for (const x of xs) if (x !== null && x !== undefined) return x; return null; },
    reverse: (a) => { expect('reverse', a, 'array', 'string'); return typeof a === 'string' ? Array.from(a).reverse().join('') : a.slice().reverse(); },
    sort: (a) => { expect('sort', a, 'array'); return sortValues('sort', a); },
    sort_by: (a, f) => {
      expect('sort_by', a, 'array'); expectRef('sort_by', f);
      const keyed = a.map((i, idx) => ({ i, k: visit(f.expref, i), idx }));
      const kinds = new Set(keyed.map((e) => typeName(e.k)));
      if (keyed.length && (kinds.size > 1 || !(kinds.has('number') || kinds.has('string')))) fail('Invalid type for sort_by(): keys must be all numbers or all strings');
      keyed.sort((x, y) => (x.k < y.k ? -1 : x.k > y.k ? 1 : x.idx - y.idx));
      return keyed.map((e) => e.i);
    },
    starts_with: (s, x) => { expect('starts_with', s, 'string'); expect('starts_with', x, 'string'); return s.startsWith(x); },
    sum: (a) => { expect('sum', a, 'array'); for (const i of a) expect('sum', i, 'number'); return a.reduce((x, y) => x + y, 0); },
    to_array: (a) => (Array.isArray(a) ? a : [a]),
    to_number: (a) => { if (typeof a === 'number') return a; if (typeof a === 'string') { const n = Number(a); return a.trim() !== '' && Number.isFinite(n) ? n : null; } return null; },
    to_string: (a) => (typeof a === 'string' ? a : JSON.stringify(a)),
    type: (a) => typeName(a),
    values: (o) => { expect('values', o, 'object'); return Object.keys(o).map((k) => o[k]); },
  };

  function expectRef(name, f) {
    if (f === null || typeof f !== 'object' || !('expref' in f)) fail('Invalid type for ' + name + '(): expected an expression reference');
  }
  function extreme(name, a, better) {
    const kinds = new Set(a.map(typeName));
    if (kinds.size > 1 || !(kinds.has('number') || kinds.has('string'))) fail('Invalid type for ' + name + '(): elements must be all numbers or all strings');
    return a.reduce((best, x) => (better(x, best) ? x : best));
  }
  function extremeBy(name, a, f, better) {
    if (a.length === 0) return null;
    const keyed = a.map((i) => ({ i, k: visit(f.expref, i) }));
    const kinds = new Set(keyed.map((e) => typeName(e.k)));
    if (kinds.size > 1 || !(kinds.has('number') || kinds.has('string'))) fail('Invalid type for ' + name + '(): keys must be all numbers or all strings');
    return keyed.reduce((best, e) => (better(e.k, best.k) ? e : best)).i;
  }
  function sortValues(name, a) {
    const kinds = new Set(a.map(typeName));
    if (a.length && (kinds.size > 1 || !(kinds.has('number') || kinds.has('string')))) fail('Invalid type for ' + name + '(): elements must be all numbers or all strings');
    return a.slice().sort((x, y) => (x < y ? -1 : x > y ? 1 : 0));
  }
  function callFunction(name, args) {
    const fn = Object.prototype.hasOwnProperty.call(FUNCTIONS, name) ? FUNCTIONS[name] : undefined;
    if (fn === undefined) fail('Unknown function: ' + name + '()');
    return fn(...args);
  }

  const cache = new Map();
  internal.jmespath = Object.freeze(function jmespath(data, query) {
    if (typeof query !== 'string') fail('The JMESPath query must be a string');
    let ast = cache.get(query);
    if (ast === undefined) {
      ast = parse(query);
      if (cache.size >= 64) cache.clear();
      cache.set(query, ast);
    }
    return visit(ast, data);
  });
})();
