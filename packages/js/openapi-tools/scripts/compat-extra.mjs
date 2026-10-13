// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * The compatibility changes of VER-2 that oasdiff cannot see because they live in `x-orcon-*`
 * extensions (SPEC-012 Amendment 2, section 2-5). It compares two bundles, after the
 * experimental operations are removed, and reports:
 *
 *   - a stricter `x-orcon-permission` on an operation or an event;
 *   - `x-orcon-idempotency` going from optional to required;
 *   - a changed `x-orcon-sort-default` (an observable default);
 *   - a problem code removed from the catalog, or its status changed;
 *   - an operation that replaces a problem code by another of the same status (the code
 *     changed for the same condition);
 *   - a changed `cloudEventsType` of an event.
 *
 *   node scripts/compat-extra.mjs --base <bundle> --head <bundle>      (exit 1 on a finding)
 */

import { must, operationsOf, problemCatalog, readDocument } from './doc-io.mjs';
import { isMain, parseArgs } from './lib.mjs';
import { withoutExperimental } from './stability-filter.mjs';

/** The permission requirement as a list of alternatives; each alternative is a set of names. */
export function alternatives(value) {
  if (typeof value === 'string') return value === 'authenticated' || value === 'anonymous' ? [new Set()] : [new Set([value])];
  if (Array.isArray(value?.allOf)) return [new Set(value.allOf)];
  if (Array.isArray(value?.anyOf)) return value.anyOf.map((p) => new Set([p]));
  return [new Set()];
}

/** True when `head` asks for something a caller allowed by `base` might not hold. */
export function permissionStricter(base, head) {
  const headAlternatives = alternatives(head);
  return alternatives(base).some((b) => !headAlternatives.some((h) => [...h].every((p) => b.has(p))));
}

const opKey = (o) => `${o.kind}:${o.method}:${o.key}`;

/** Findings between two bundled documents: [{ where, message }]. */
export function compatExtra(baseDoc, headDoc) {
  const base = withoutExperimental(baseDoc);
  const head = withoutExperimental(headDoc);
  const findings = [];
  const headOps = new Map(operationsOf(head).map((o) => [opKey(o), o]));
  const catalogBase = problemCatalog(base);
  const catalogHead = problemCatalog(head);

  for (const b of operationsOf(base)) {
    const h = headOps.get(opKey(b));
    if (!h) continue; // a removed operation is oasdiff's finding
    const where = b.kind === 'webhook' ? `event ${b.key}` : `${b.method.toUpperCase()} ${b.key}`;
    const permissionKey = b.kind === 'webhook' ? undefined : 'x-orcon-permission';
    if (permissionKey && b.op[permissionKey] !== undefined && h.op[permissionKey] !== undefined && permissionStricter(b.op[permissionKey], h.op[permissionKey])) {
      findings.push({ where, message: 'x-orcon-permission is stricter than in the base' });
    }
    if (b.kind === 'webhook') {
      const be = b.op['x-orcon-event'] ?? {};
      const he = h.op['x-orcon-event'] ?? {};
      if (be.cloudEventsType && he.cloudEventsType && be.cloudEventsType !== he.cloudEventsType) {
        findings.push({ where, message: `cloudEventsType changed from ${be.cloudEventsType} to ${he.cloudEventsType} (a new version is a new event type)` });
      }
      if (be.permission && he.permission && permissionStricter(be.permission, he.permission)) {
        findings.push({ where, message: 'the permission that receives the event is stricter than in the base' });
      }
      continue;
    }
    if (b.op['x-orcon-idempotency'] === 'optional' && h.op['x-orcon-idempotency'] === 'required') {
      findings.push({ where, message: 'x-orcon-idempotency went from optional to required' });
    }
    const sortDefault = (o) => (o.op.parameters ?? []).find((p) => p.name === 'sort')?.['x-orcon-sort-default'];
    if (sortDefault(b) !== undefined && sortDefault(h) !== undefined && sortDefault(b) !== sortDefault(h)) {
      findings.push({ where, message: `the default sort changed from ${sortDefault(b)} to ${sortDefault(h)}` });
    }
    const removed = (b.op['x-orcon-problems'] ?? []).filter((c) => !(h.op['x-orcon-problems'] ?? []).includes(c));
    const added = (h.op['x-orcon-problems'] ?? []).filter((c) => !(b.op['x-orcon-problems'] ?? []).includes(c));
    for (const code of removed) {
      const status = catalogBase.get(code)?.status;
      const replacement = added.find((a) => catalogHead.get(a)?.status === status);
      if (replacement) findings.push({ where, message: `problem code ${code} was replaced by ${replacement} for the same status ${status}` });
    }
  }
  for (const [code, info] of catalogBase) {
    const next = catalogHead.get(code);
    if (!next) findings.push({ where: `problem code ${code}`, message: 'was removed from the catalog' });
    else if (next.status !== info.status) findings.push({ where: `problem code ${code}`, message: `status changed from ${info.status} to ${next.status}` });
  }
  return findings;
}

if (isMain(import.meta.url)) {
  const { flags } = parseArgs(process.argv.slice(2));
  must(flags.base && flags.head, 'usage: compat-extra.mjs --base <bundle> --head <bundle>');
  const findings = compatExtra(readDocument(flags.base), readDocument(flags.head));
  for (const f of findings) process.stdout.write(`compat-extra: ${f.where}: ${f.message}\n`);
  if (findings.length > 0) {
    process.stdout.write(`compat-extra: ${findings.length} breaking change(s) that oasdiff does not detect\n`);
    process.exit(1);
  }
  process.stdout.write('compat-extra: no breaking change\n');
}
