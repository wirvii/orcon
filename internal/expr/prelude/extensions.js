// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0
//
// n8n-compatible extension methods (ADR-0018 section 1): the documented
// helper methods on strings, numbers, booleans, arrays, objects and dates.
// They are written from the public description of their behaviour. A method
// the language already has is never replaced. They are installed as
// non-enumerable properties of the built-in prototypes, so they do not appear
// in iteration or in JSON output.
//
// Known difference: because they live on the prototypes, reading (without
// calling) an extension name that the data lacks, for example $json.compact on
// an object without that field, yields the method instead of undefined.
(function () {
  'use strict';
  const internal = globalThis.__orcon;
  const host = internal.host;
  const { DateTime, Duration } = globalThis.luxon;
  const OP_HASH = 8;

  function def(proto, name, fn) {
    if (name in proto) return;
    Object.defineProperty(proto, name, { value: fn, writable: true, configurable: true, enumerable: false });
  }
  function defAll(proto, table) {
    for (const name of Object.keys(table)) def(proto, name, table[name]);
  }
  const isObject = (v) => v !== null && typeof v === 'object' && !Array.isArray(v);
  const str = (v) => String(v);

  // ---- strings --------------------------------------------------------------

  const EMAIL = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
  const DOMAIN = /^(?:[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?\.)+[A-Za-z]{2,63}$/;
  const URL_RE = /^(?:[a-z][a-z0-9+.-]*):\/\/[^\s/?#]+(?:[/?#][^\s]*)?$/i;

  function bytesToBase64(text) {
    return btoa(unescape(encodeURIComponent(text)));
  }
  function base64ToBytes(b64) {
    return decodeURIComponent(escape(atob(b64)));
  }
  function words(s) {
    return s.split(/\s+/).filter((w) => w.length > 0);
  }

  defAll(String.prototype, {
    base64Encode() { return bytesToBase64(str(this)); },
    base64Decode() { return base64ToBytes(str(this)); },
    extractDomain() {
      const s = str(this);
      const m = /^(?:[a-z][a-z0-9+.-]*:\/\/)?(?:[^@/?#]*@)?([^/:?#\s]+)/i.exec(s);
      if (m && DOMAIN.test(m[1])) return m[1];
      const e = /@([A-Za-z0-9.-]+\.[A-Za-z]{2,})/.exec(s);
      return e ? e[1] : undefined;
    },
    extractEmail() {
      const m = /[^\s<>()[\]\\,;:"@]+@[^\s<>()[\]\\,;:"@]+\.[A-Za-z]{2,}/.exec(str(this));
      return m ? m[0] : undefined;
    },
    extractUrl() {
      const m = /[a-z][a-z0-9+.-]*:\/\/[^\s"'<>]+/i.exec(str(this));
      return m ? m[0] : undefined;
    },
    extractUrlPath() {
      const m = /^[a-z][a-z0-9+.-]*:\/\/[^/?#\s]+([^?#\s]*)/i.exec(str(this));
      return m ? m[1] : undefined;
    },
    hash(algo) {
      const name = (algo === undefined ? 'md5' : str(algo)).toLowerCase();
      const out = host(OP_HASH, name + '\t' + str(this));
      if (out === undefined) throw new internal.ExpressionError('Unsupported hash algorithm: ' + name);
      return out;
    },
    isDomain() { return DOMAIN.test(str(this)); },
    isEmail() { return EMAIL.test(str(this)); },
    isEmpty() { return str(this) === ''; },
    isNotEmpty() { return str(this) !== ''; },
    isNumeric() { const s = str(this).trim(); return s !== '' && Number.isFinite(Number(s)); },
    isUrl() { return URL_RE.test(str(this)); },
    parseJson() { return JSON.parse(str(this)); },
    quote(mark) { const q = mark === undefined ? '"' : str(mark); return q + str(this).split(q).join('\\' + q) + q; },
    removeMarkdown() {
      return str(this)
        .replace(/```[\s\S]*?```/g, (m) => m.replace(/^```[^\n]*\n?|```$/g, ''))
        .replace(/!\[([^\]]*)\]\([^)]*\)/g, '$1')
        .replace(/\[([^\]]*)\]\([^)]*\)/g, '$1')
        .replace(/^#{1,6}\s+/gm, '')
        .replace(/(\*\*|__)(.*?)\1/g, '$2')
        .replace(/(\*|_)(.*?)\1/g, '$2')
        .replace(/~~(.*?)~~/g, '$1')
        .replace(/`([^`]*)`/g, '$1')
        .replace(/^>\s?/gm, '')
        .replace(/^[-*+]\s+/gm, '')
        .trim();
    },
    removeTags() { return str(this).replace(/<[^>]*>/g, ''); },
    replaceSpecialChars() { return str(this).normalize('NFD').replace(/[̀-ͯ]/g, ''); },
    toBoolean() {
      const s = str(this).trim().toLowerCase();
      if (['true', 'yes', '1', 'y', 't'].includes(s)) return true;
      if (['false', 'no', '0', 'n', 'f', ''].includes(s)) return false;
      throw new internal.ExpressionError('Cannot convert "' + str(this) + '" to a boolean');
    },
    toDate() { return DateTime.fromJSDate(new Date(str(this))).toJSDate(); },
    toDateTime(format) {
      const s = str(this);
      let dt = format === undefined ? DateTime.fromISO(s, { setZone: true }) : DateTime.fromFormat(s, str(format));
      if (!dt.isValid && format === undefined) dt = DateTime.fromRFC2822(s, { setZone: true });
      if (!dt.isValid && format === undefined) dt = DateTime.fromSQL(s, { setZone: true });
      if (!dt.isValid) throw new internal.ExpressionError('Invalid date: ' + s);
      return dt;
    },
    toDecimalNumber() { return Number.parseFloat(str(this)); },
    toFloat() { return Number.parseFloat(str(this)); },
    toInt(radix) { return Number.parseInt(str(this), radix === undefined ? 10 : radix); },
    toWholeNumber() { return Math.trunc(Number.parseFloat(str(this))); },
    toSentenceCase() {
      const s = str(this).toLowerCase();
      return s.replace(/(^\s*|[.!?]\s+)([a-zà-ɏ])/g, (m, a, b) => a + b.toUpperCase());
    },
    toSnakeCase() {
      return str(this)
        .replace(/([a-z0-9])([A-Z])/g, '$1 $2')
        .toLowerCase()
        .replace(/[^a-z0-9]+/g, '_')
        .replace(/^_+|_+$/g, '');
    },
    toTitleCase() {
      return str(this).replace(/\S+/g, (w) => w.charAt(0).toUpperCase() + w.slice(1).toLowerCase());
    },
    urlDecode(entireString) { return entireString === true ? decodeURI(str(this)) : decodeURIComponent(str(this)); },
    urlEncode(entireString) { return entireString === true ? encodeURI(str(this)) : encodeURIComponent(str(this)); },
  });
  void words;

  // ---- numbers and booleans ------------------------------------------------------

  defAll(Number.prototype, {
    abs() { return Math.abs(Number(this)); },
    ceil() { return Math.ceil(Number(this)); },
    floor() { return Math.floor(Number(this)); },
    format(locale, options) { return Number(this).toLocaleString(locale, options); },
    isEven() { const n = Number(this); return Number.isInteger(n) && n % 2 === 0; },
    isOdd() { const n = Number(this); return Number.isInteger(n) && Math.abs(n % 2) === 1; },
    round(decimals) {
      const n = Number(this);
      if (decimals === undefined) return Math.round(n);
      const f = Math.pow(10, decimals);
      return Math.round((n + Number.EPSILON) * f) / f;
    },
    toBoolean() { return Number(this) !== 0; },
    toDateTime(unit) {
      const n = Number(this);
      return unit === 's' || unit === 'seconds' ? DateTime.fromSeconds(n) : DateTime.fromMillis(n);
    },
  });
  defAll(Boolean.prototype, {
    toInt() { return this.valueOf() ? 1 : 0; },
  });

  // ---- arrays ------------------------------------------------------------------

  const same = (a, b) => JSON.stringify(a) === JSON.stringify(b);
  function numbers(arr, op) {
    const out = [];
    for (const v of arr) {
      const n = typeof v === 'number' ? v : Number(v);
      if (!Number.isFinite(n) || v === '' || v === null || typeof v === 'boolean') {
        throw new internal.ExpressionError('Elements in array must be numbers for ' + op + '() (found ' + JSON.stringify(v) + ')');
      }
      out.push(n);
    }
    return out;
  }

  defAll(Array.prototype, {
    append(...items) { return this.concat(items); },
    average() { const n = numbers(this, 'average'); return n.length === 0 ? NaN : n.reduce((a, b) => a + b, 0) / n.length; },
    chunk(size) {
      const out = [];
      for (let i = 0; i < this.length; i += size) out.push(this.slice(i, i + size));
      return out;
    },
    compact() { return this.filter((v) => v !== null && v !== undefined && v !== '' && !(typeof v === 'number' && Number.isNaN(v))); },
    difference(other) { return this.filter((v) => !other.some((o) => same(o, v))); },
    first() { return this[0]; },
    intersection(other) { return this.filter((v) => other.some((o) => same(o, v))).filter((v, i, a) => a.findIndex((x) => same(x, v)) === i); },
    isEmpty() { return this.length === 0; },
    isNotEmpty() { return this.length > 0; },
    last() { return this[this.length - 1]; },
    max() { const n = numbers(this, 'max'); return n.length === 0 ? NaN : Math.max(...n); },
    merge(other) { return this.map((v, i) => (isObject(v) && isObject(other[i]) ? Object.assign({}, v, other[i]) : v)); },
    min() { const n = numbers(this, 'min'); return n.length === 0 ? NaN : Math.min(...n); },
    pluck(...fields) {
      return this.map((o) => {
        if (fields.length === 1) return o == null ? undefined : o[fields[0]];
        const out = [];
        for (const f of fields) out.push(o == null ? undefined : o[f]);
        return out;
      });
    },
    randomItem() { return this[Math.floor(Math.random() * this.length)]; },
    removeDuplicates(keys) {
      if (keys === undefined || keys === '') return this.filter((v, i, a) => a.findIndex((x) => same(x, v)) === i);
      const names = String(keys).split(',').map((k) => k.trim());
      const seen = [];
      return this.filter((o) => {
        const key = names.map((k) => (o == null ? undefined : o[k]));
        if (seen.some((s) => same(s, key))) return false;
        seen.push(key);
        return true;
      });
    },
    renameKeys(...pairs) {
      return this.map((o) => {
        const out = {};
        for (const k of Object.keys(o)) {
          const i = pairs.indexOf(k);
          out[i >= 0 && i % 2 === 0 ? pairs[i + 1] : k] = o[k];
        }
        return out;
      });
    },
    smartJoin(keyField, nameField) {
      const out = {};
      for (const o of this) {
        if (o != null && typeof o[keyField] === 'string' && o[nameField] !== undefined) out[o[keyField]] = o[nameField];
      }
      return out;
    },
    sum() { return numbers(this, 'sum').reduce((a, b) => a + b, 0); },
    toJsonString() { return JSON.stringify(this); },
    union(other) { return this.concat(other).filter((v, i, a) => a.findIndex((x) => same(x, v)) === i); },
    unique(keys) { return this.removeDuplicates(keys); },
  });

  // ---- objects -----------------------------------------------------------------

  defAll(Object.prototype, {
    compact() {
      const out = {};
      for (const k of Object.keys(this)) {
        const v = this[k];
        if (v !== null && v !== undefined && v !== '' && !(typeof v === 'number' && Number.isNaN(v))) out[k] = v;
      }
      return out;
    },
    hasField(name) { return Object.prototype.hasOwnProperty.call(this, name); },
    isEmpty() { return Object.keys(this).length === 0; },
    isNotEmpty() { return Object.keys(this).length > 0; },
    keepFieldsContaining(text) {
      const out = {};
      for (const k of Object.keys(this)) {
        if (JSON.stringify(this[k]).includes(text)) out[k] = this[k];
      }
      return out;
    },
    merge(other) { return Object.assign({}, other, this); },
    removeField(name) {
      const out = Object.assign({}, this);
      delete out[name];
      return out;
    },
    removeFieldsContaining(text) {
      const out = {};
      for (const k of Object.keys(this)) {
        if (!JSON.stringify(this[k]).includes(text)) out[k] = this[k];
      }
      return out;
    },
    toJsonString() { return JSON.stringify(this); },
    urlEncode() {
      return Object.keys(this).map((k) => encodeURIComponent(k) + '=' + encodeURIComponent(this[k])).join('&');
    },
  });

  // ---- dates ----------------------------------------------------------------------

  const UNITS = {
    year: 'year', years: 'year', quarter: 'quarter', quarters: 'quarter', month: 'month', months: 'month',
    week: 'week', weeks: 'week', day: 'day', days: 'day', hour: 'hour', hours: 'hour',
    minute: 'minute', minutes: 'minute', second: 'second', seconds: 'second',
    millisecond: 'millisecond', milliseconds: 'millisecond',
  };
  const unitOf = (u) => {
    const unit = UNITS[u];
    if (unit === undefined) throw new internal.ExpressionError('Unsupported unit: ' + u);
    return unit;
  };
  const toDT = (d) => (d instanceof DateTime ? d : DateTime.fromJSDate(d));
  // The date methods work on both JavaScript dates and Luxon DateTime values.
  const dateMethods = {
    beginningOf(unit) { return toDT(this).startOf(unitOf(unit)); },
    endOfMonth() { return toDT(this).endOf('month'); },
    extract(unit) {
      const dt = toDT(this);
      switch (unit === undefined ? 'day' : unit) {
        case 'year': return dt.year;
        case 'month': return dt.month;
        case 'week': return dt.weekNumber;
        case 'day': case 'dayOfMonth': return dt.day;
        case 'hour': return dt.hour;
        case 'minute': return dt.minute;
        case 'second': return dt.second;
        case 'timestamp': return dt.toMillis();
        default: throw new internal.ExpressionError('Unsupported unit: ' + unit);
      }
    },
    format(fmt) { return toDT(this).toFormat(fmt === undefined ? 'yyyy-MM-dd' : str(fmt)); },
    isDst() { return toDT(this).isInDST; },
    isInLast(n, unit) {
      const dt = toDT(this);
      const now = DateTime.now();
      const from = now.minus({ [unitOf(unit === undefined ? 'minutes' : unit) + 's']: n });
      return dt >= from && dt <= now;
    },
    // Luxon's DateTime already has an isWeekend getter, so calls of the name are
    // renamed to this one when the expression is prepared (expr/prepare.go).
    __isWeekend() { return toDT(this).weekday > 5; },
    minus(n, unit) { return toDT(this).minus({ [unitOf(unit === undefined ? 'milliseconds' : unit) + 's']: n }); },
    plus(n, unit) { return toDT(this).plus({ [unitOf(unit === undefined ? 'milliseconds' : unit) + 's']: n }); },
    toDateTime() { return toDT(this); },
    diffTo(other, unit) {
      const u = unitOf(unit === undefined ? 'days' : unit);
      return toDT(this).diff(toDT(other instanceof Date || other instanceof DateTime ? other : DateTime.fromISO(str(other))), u).get(u + 's');
    },
  };
  defAll(Date.prototype, dateMethods);
  defAll(DateTime.prototype, dateMethods);
  void Duration;
})();
