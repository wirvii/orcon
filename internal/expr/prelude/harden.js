// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0
//
// Last script of the prelude. It removes the private namespace and freezes
// every object reachable from the global object, so no expression can change
// a built-in, add a global or leave state for the next evaluation of the same
// runtime (ADR-0018 section 3: instances are reset between uses). The state
// the runtime itself keeps lives in closure variables, which freezing does
// not touch.
(function () {
  'use strict';
  delete globalThis.__orcon;
  // Luxon's Settings object can change the default zone and the clock of the
  // whole runtime. The expression context offers DateTime, Duration and
  // Interval only, so the namespace that reaches Settings is dropped (the var
  // declaration makes the property non-configurable; it can still be emptied).
  const luxon = globalThis.luxon;
  globalThis.luxon = undefined;

  const seen = new Set();
  // The Luxon classes are reachable from expressions through the context, not
  // through a global, so they are roots of their own.
  const pending = [globalThis, luxon];
  while (pending.length > 0) {
    const obj = pending.pop();
    if (seen.has(obj)) continue;
    seen.add(obj);
    for (const key of Reflect.ownKeys(obj)) {
      const desc = Object.getOwnPropertyDescriptor(obj, key);
      for (const part of [desc.value, desc.get, desc.set]) {
        if ((typeof part === 'object' && part !== null) || typeof part === 'function') pending.push(part);
      }
    }
    const proto = Object.getPrototypeOf(obj);
    if (proto !== null) pending.push(proto);
  }
  for (const obj of seen) Object.freeze(obj);
})();
