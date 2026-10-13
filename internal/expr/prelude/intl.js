// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0
//
// Minimal Intl for the expression sandbox (ADR-0018). QuickJS ships no Intl,
// and Luxon needs one for time zones and date formatting. This file provides
// the English (en-US) subset those uses need; any other locale is rejected
// with a RangeError instead of being formatted wrongly. Time zone data comes
// from the host (the Go time zone database) through the zone-fields operation.
//
// First script of the prelude: it takes the host function out of the global
// scope and parks it in globalThis.__orcon, which harden.js deletes.
(function () {
  'use strict';
  const host = globalThis.__host;
  delete globalThis.__host;
  const internal = { host };
  Object.defineProperty(globalThis, '__orcon', { value: internal, configurable: true, writable: true });

  const OP_ZONE_FIELDS = 7;
  const MONTHS = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December'];
  const DAYS = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'];

  function checkLocale(locales) {
    const l = Array.isArray(locales) ? locales[0] : locales;
    if (l === undefined || l === null) return 'en-US';
    const s = String(l);
    if (s === 'en' || s.startsWith('en-') || s.startsWith('en_')) return s.startsWith('en-u-') ? 'en-US' : s.split('-u-')[0].replace('_', '-');
    throw new RangeError('locale ' + s + ' is not available in the expression sandbox; only English locales are supported');
  }

  // fields returns [year, month, day, hour, minute, second, weekday] of the
  // instant ms in the zone, or null when the zone is unknown.
  function fields(zone, ms) {
    if (zone === undefined || zone === 'UTC' || zone === 'Etc/UTC' || zone === 'GMT') {
      const d = new Date(ms);
      return [d.getUTCFullYear(), d.getUTCMonth() + 1, d.getUTCDate(), d.getUTCHours(), d.getUTCMinutes(), d.getUTCSeconds(), d.getUTCDay()];
    }
    const r = host(OP_ZONE_FIELDS, zone + '\t' + ms);
    if (!r) return null;
    const f = r.split(' ').map(Number);
    f.push(new Date(Date.UTC(f[0], f[1] - 1, f[2])).getUTCDay());
    return f;
  }

  function pad(n, w) { return String(n).padStart(w, '0'); }

  const DATE_STYLE = {
    full: { weekday: 'long', year: 'numeric', month: 'long', day: 'numeric' },
    long: { year: 'numeric', month: 'long', day: 'numeric' },
    medium: { year: 'numeric', month: 'short', day: 'numeric' },
    short: { year: '2-digit', month: 'numeric', day: 'numeric' },
  };
  const TIME_STYLE = {
    full: { hour: 'numeric', minute: '2-digit', second: '2-digit', timeZoneName: 'long' },
    long: { hour: 'numeric', minute: '2-digit', second: '2-digit', timeZoneName: 'short' },
    medium: { hour: 'numeric', minute: '2-digit', second: '2-digit' },
    short: { hour: 'numeric', minute: '2-digit' },
  };

  class DateTimeFormat {
    constructor(locales, options) {
      this._locale = checkLocale(locales);
      const o = Object.assign({}, options);
      if (o.timeZone !== undefined) {
        const z = String(o.timeZone);
        if (fields(z, 0) === null) throw new RangeError('Invalid time zone specified: ' + z);
        o.timeZone = z;
      }
      if (o.dateStyle || o.timeStyle) {
        Object.assign(o, o.dateStyle ? DATE_STYLE[o.dateStyle] : {}, o.timeStyle ? TIME_STYLE[o.timeStyle] : {});
      }
      const has = ['weekday', 'era', 'year', 'month', 'day', 'hour', 'minute', 'second', 'timeZoneName'].some((k) => o[k] !== undefined);
      if (!has) { o.year = 'numeric'; o.month = 'numeric'; o.day = 'numeric'; }
      this._o = o;
    }

    resolvedOptions() {
      return { locale: this._locale, calendar: 'gregory', numberingSystem: 'latn', timeZone: this._o.timeZone || 'UTC' };
    }

    formatToParts(date) {
      const ms = date === undefined ? Date.now() : (date instanceof Date ? date.getTime() : Number(date));
      if (!Number.isFinite(ms)) throw new RangeError('Invalid time value');
      const o = this._o;
      const f = fields(o.timeZone, ms);
      let year = f[0];
      let era = 'AD';
      if (year <= 0) { era = 'BC'; year = 1 - year; }
      const parts = [];
      const lit = (v) => parts.push({ type: 'literal', value: v });
      const textual = o.month === 'long' || o.month === 'short' || o.month === 'narrow';
      const dateFields = o.year !== undefined || o.month !== undefined || o.day !== undefined || o.weekday !== undefined;
      const timeFields = o.hour !== undefined || o.minute !== undefined || o.second !== undefined;
      const num = (type, style, v) => {
        const two = style === '2-digit';
        parts.push({ type, value: two ? pad(v % (type === 'year' ? 100 : 1000), 2) : String(v) });
      };
      const monthPart = () => {
        if (textual) {
          const name = MONTHS[f[1] - 1];
          parts.push({ type: 'month', value: o.month === 'long' ? name : (o.month === 'short' ? name.slice(0, 3) : name[0]) });
        } else {
          num('month', o.month, f[1]);
        }
      };
      const weekdayPart = () => {
        const name = DAYS[f[6]];
        parts.push({ type: 'weekday', value: o.weekday === 'long' ? name : (o.weekday === 'short' ? name.slice(0, 3) : name[0]) });
      };
      if (dateFields) {
        if (textual) {
          if (o.weekday) { weekdayPart(); lit(', '); }
          monthPart();
          if (o.day !== undefined) { lit(' '); num('day', o.day, f[2]); }
          if (o.year !== undefined) { lit(o.day !== undefined ? ', ' : ' '); num('year', o.year, year); }
        } else {
          if (o.weekday) { weekdayPart(); if (o.month !== undefined || o.day !== undefined || o.year !== undefined) lit(', '); }
          const seq = [];
          if (o.month !== undefined) seq.push(() => monthPart());
          if (o.day !== undefined) seq.push(() => num('day', o.day, f[2]));
          if (o.year !== undefined) seq.push(() => num('year', o.year, year));
          seq.forEach((fn, i) => { if (i > 0) lit('/'); fn(); });
        }
        if (o.era !== undefined) { lit(' '); parts.push({ type: 'era', value: era }); }
      }
      if (timeFields) {
        if (dateFields) lit(', ');
        const h12 = o.hour12 !== false && o.hourCycle !== 'h23' && o.hourCycle !== 'h24';
        let h = f[3];
        let suffix = '';
        if (o.hour !== undefined) {
          if (h12) { suffix = h >= 12 ? 'PM' : 'AM'; h = h % 12 || 12; }
          parts.push({ type: 'hour', value: o.hour === '2-digit' || (!h12 && o.minute !== undefined) ? pad(h, 2) : String(h) });
        }
        if (o.minute !== undefined) {
          if (o.hour !== undefined) lit(':');
          parts.push({ type: 'minute', value: o.hour !== undefined || o.minute === '2-digit' ? pad(f[4], 2) : String(f[4]) });
        }
        if (o.second !== undefined) {
          if (o.hour !== undefined || o.minute !== undefined) lit(':');
          parts.push({ type: 'second', value: o.minute !== undefined || o.second === '2-digit' ? pad(f[5], 2) : String(f[5]) });
        }
        if (suffix) { lit(' '); parts.push({ type: 'dayPeriod', value: suffix }); }
      }
      if (o.timeZoneName !== undefined) {
        lit(' ');
        const z = o.timeZone || 'UTC';
        let name = 'UTC';
        if (z !== 'UTC' && z !== 'Etc/UTC') {
          const local = Date.UTC(f[0], f[1] - 1, f[2], f[3], f[4], f[5]);
          const off = Math.round((local - Math.floor(ms / 1000) * 1000) / 60000);
          const sign = off < 0 ? '-' : '+';
          const a = Math.abs(off);
          name = off === 0 ? 'GMT' : 'GMT' + sign + Math.floor(a / 60) + (a % 60 ? ':' + pad(a % 60, 2) : '');
        }
        parts.push({ type: 'timeZoneName', value: name });
      }
      return parts;
    }

    format(date) {
      return this.formatToParts(date).map((p) => p.value).join('');
    }
  }

  function group(intPart) {
    return intPart.replace(/\B(?=(\d{3})+(?!\d))/g, ',');
  }

  class NumberFormat {
    constructor(locales, options) {
      this._locale = checkLocale(locales);
      this._o = Object.assign({}, options);
    }

    resolvedOptions() { return Object.assign({ locale: this._locale, numberingSystem: 'latn' }, this._o); }

    format(value) {
      const o = this._o;
      let n = Number(value);
      if (Number.isNaN(n)) return 'NaN';
      if (!Number.isFinite(n)) return (n < 0 ? '-' : '') + '∞';
      const style = o.style || 'decimal';
      if (style === 'percent') n *= 100;
      const currency = style === 'currency';
      const minFrac = o.minimumFractionDigits !== undefined ? o.minimumFractionDigits : (currency ? 2 : 0);
      const maxFrac = o.maximumFractionDigits !== undefined ? Math.max(o.maximumFractionDigits, minFrac) : Math.max(minFrac, currency ? 2 : (style === 'percent' ? 0 : 3));
      const neg = n < 0;
      let s = Math.abs(n).toFixed(maxFrac);
      if (s.indexOf('.') >= 0) {
        s = s.replace(/0+$/, '');
        const dot = s.indexOf('.');
        const have = s.length - dot - 1;
        if (have < minFrac) s += '0'.repeat(minFrac - have);
        if (s.endsWith('.')) s = s.slice(0, -1);
      }
      let [ip, fp] = s.split('.');
      if (o.minimumIntegerDigits) ip = ip.padStart(o.minimumIntegerDigits, '0');
      if (o.useGrouping !== false) ip = group(ip);
      let out = fp ? ip + '.' + fp : ip;
      if (currency) {
        const symbols = { USD: '$', EUR: '€', GBP: '£', JPY: '¥', INR: '₹', CNY: 'CN¥', KRW: '₩', MXN: 'MX$', CAD: 'CA$', AUD: 'A$', BRL: 'R$', COP: 'COP ' };
        const code = String(o.currency || '').toUpperCase();
        out = (symbols[code] !== undefined ? symbols[code] : code + ' ') + out;
      } else if (style === 'percent') {
        out += '%';
      }
      return (neg ? '-' : '') + out;
    }
  }

  const UNITS = ['second', 'minute', 'hour', 'day', 'week', 'month', 'quarter', 'year'];
  class RelativeTimeFormat {
    constructor(locales, options) {
      this._locale = checkLocale(locales);
      this._o = Object.assign({ numeric: 'always', style: 'long' }, options);
    }

    format(value, unit) {
      const u = String(unit).replace(/s$/, '');
      if (!UNITS.includes(u)) throw new RangeError('Invalid unit argument for format() \'' + unit + '\'');
      const v = Number(value);
      if (this._o.numeric === 'auto') {
        const special = { day: { '-1': 'yesterday', '0': 'today', '1': 'tomorrow' }, year: { '-1': 'last year', '0': 'this year', '1': 'next year' }, month: { '-1': 'last month', '0': 'this month', '1': 'next month' }, week: { '-1': 'last week', '0': 'this week', '1': 'next week' }, hour: { '0': 'this hour' }, minute: { '0': 'this minute' }, second: { '0': 'now' } };
        const hit = special[u] && special[u][String(v)];
        if (hit) return hit;
      }
      const a = Math.abs(v);
      const body = new NumberFormat('en-US').format(a) + ' ' + u + (a === 1 ? '' : 's');
      return v < 0 || Object.is(v, -0) ? body + ' ago' : 'in ' + body;
    }
  }

  class ListFormat {
    constructor(locales, options) {
      this._locale = checkLocale(locales);
      this._o = Object.assign({ type: 'conjunction', style: 'long' }, options);
    }

    format(list) {
      const a = Array.from(list, String);
      if (a.length <= 1) return a.join('');
      const word = this._o.type === 'disjunction' ? 'or' : 'and';
      if (this._o.type === 'unit') return a.join(this._o.style === 'narrow' ? ' ' : ', ');
      if (a.length === 2) return a[0] + ' ' + word + ' ' + a[1];
      return a.slice(0, -1).join(', ') + ', ' + word + ' ' + a[a.length - 1];
    }
  }

  const Intl = {};
  for (const [name, value] of Object.entries({ DateTimeFormat, NumberFormat, RelativeTimeFormat, ListFormat })) {
    Object.defineProperty(Intl, name, { value, writable: true, configurable: true });
  }
  Object.defineProperty(globalThis, 'Intl', { value: Intl, writable: true, configurable: true });

  // The locale-sensitive methods of the built-ins delegate to the shim.
  const dateDefaults = (opts, kind) => {
    const o = Object.assign({}, opts);
    if (o.dateStyle !== undefined || o.timeStyle !== undefined) return o;
    const hasDate = ['weekday', 'year', 'month', 'day'].some((k) => o[k] !== undefined);
    const hasTime = ['hour', 'minute', 'second'].some((k) => o[k] !== undefined);
    const needDate = kind === 'date' ? !hasDate : kind === 'any' && !hasDate && !hasTime;
    const needTime = kind === 'time' ? !hasTime : kind === 'any' && !hasDate && !hasTime;
    if (needDate) { o.year = 'numeric'; o.month = 'numeric'; o.day = 'numeric'; }
    if (needTime) { o.hour = 'numeric'; o.minute = '2-digit'; o.second = '2-digit'; }
    return o;
  };
  const defineMethod = (proto, name, fn) => Object.defineProperty(proto, name, { value: fn, writable: true, configurable: true });
  defineMethod(Date.prototype, 'toLocaleDateString', function (l, o) { return new DateTimeFormat(l, dateDefaults(o, 'date')).format(this); });
  defineMethod(Date.prototype, 'toLocaleTimeString', function (l, o) { return new DateTimeFormat(l, dateDefaults(o, 'time')).format(this); });
  defineMethod(Date.prototype, 'toLocaleString', function (l, o) { return new DateTimeFormat(l, dateDefaults(o, 'any')).format(this); });
  defineMethod(Number.prototype, 'toLocaleString', function (l, o) { return new NumberFormat(l, o).format(this); });
})();
