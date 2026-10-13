// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package wazero_test

import (
	"slices"
	"sort"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/wazero"
	"github.com/wirvii/orcon/internal/expr"
)

// nativeInventory is evaluated in the guest: it walks the object graph from
// the global object (own properties, getters and setters, prototypes) and from
// the intrinsics that no global name reaches, and lists every native function
// once, named by the first path that reaches it. Functions written in
// JavaScript by the prelude, Luxon or jmespath are not native and are left out.
const nativeInventory = `(() => {
  const toString = Function.prototype.toString;
  const isNative = (f) => typeof f === 'function' && /\{\s*\[native code\]\s*\}\s*$/.test(toString.call(f));
  const found = new Map();
  const visited = new Set();
  const queue = [];
  const visit = (path, o) => {
    if (o !== null && (typeof o === 'object' || typeof o === 'function') && !visited.has(o)) {
      visited.add(o);
      queue.push([path, o]);
    }
  };
  const note = (name, f) => { if (isNative(f) && !found.has(f)) found.set(f, name); };
  visit('', globalThis);
  const intrinsics = {};
  const make = (name, f) => { try { intrinsics[name] = f(); } catch (e) { /* the engine lacks it */ } };
  make('%ArrayIterator%', () => [][Symbol.iterator]());
  make('%StringIterator%', () => ''[Symbol.iterator]());
  make('%MapIterator%', () => new Map()[Symbol.iterator]());
  make('%SetIterator%', () => new Set()[Symbol.iterator]());
  make('%RegExpStringIterator%', () => /a/g[Symbol.matchAll](''));
  make('%Generator%', () => (function* () {})());
  make('%AsyncGenerator%', () => (async function* () {})());
  make('%IteratorHelper%', () => [].values().map((x) => x));
  make('%WrapForValidIterator%', () => Iterator.from({ next() { return { done: true }; } }));
  for (const name of Object.keys(intrinsics)) visit(name, intrinsics[name]);
  make('%GeneratorFunction%', () => visit('%GeneratorFunction%', Object.getPrototypeOf(function* () {})));
  make('%AsyncFunction%', () => visit('%AsyncFunction%', Object.getPrototypeOf(async function () {})));
  make('%TypedArray%', () => visit('%TypedArray%', Object.getPrototypeOf(Int8Array)));
  while (queue.length > 0) {
    const [path, o] = queue.shift();
    let keys;
    try { keys = Reflect.ownKeys(o); } catch (e) { continue; }
    for (const k of keys) {
      const label = typeof k === 'symbol' ? '[' + k.description + ']' : k;
      const name = path === '' ? label : path + '.' + label;
      let d;
      try { d = Object.getOwnPropertyDescriptor(o, k); } catch (e) { continue; }
      if (d === undefined) continue;
      if ('value' in d) {
        note(name, d.value);
        visit(name, d.value);
      } else {
        if (d.get) note('get ' + name, d.get);
        if (d.set) note('set ' + name, d.set);
      }
    }
    let proto;
    try { proto = Object.getPrototypeOf(o); } catch (e) { proto = null; }
    if (proto !== null) visit(path + '.__proto__', proto);
  }
  return [...found.values()].sort();
})()`

// guestNatives returns the sorted names of the native functions of the guest.
func guestNatives(tb testing.TB) []string {
	tb.Helper()
	ev := evaluatorIn(tb, wazero.ModeCooperativeAbandon, expr.Options{DisableFastPath: true, Timeout: 30 * time.Second})
	s := ev.Open()
	defer func() { _ = s.Close(tb.Context()) }()
	p, err := ev.Compile("={{ " + nativeInventory + " }}")
	if err != nil {
		tb.Fatal(err)
	}
	res, err := s.Eval(tb.Context(), p, newEnv(tb, `{}`))
	if err != nil {
		tb.Fatal(err)
	}
	var names []string
	for _, item := range res.Value.Items() {
		name, _ := item.Str()
		names = append(names, name)
	}
	sort.Strings(names)
	return slices.Compact(names)
}
