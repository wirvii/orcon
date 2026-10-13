// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * The parts of the API changelog that oasdiff does not write (spec section 4.8): the table of
 * deprecations (`x-orcon-sunset`, `x-orcon-deprecation-guide`) and the separate section of
 * experimental operations. Used by scripts/changelog.mjs; it also prints both sections:
 *
 *   node scripts/changelog-deprecations.mjs --head <bundle> [--base <bundle>]
 */

import { cell, must, operationsOf, readDocument } from './doc-io.mjs';
import { isMain, parseArgs } from './lib.mjs';

const walkSchemas = (doc, visit) => {
  const seen = new Set();
  const walk = (node, path) => {
    if (!node || typeof node !== 'object' || seen.has(node)) return;
    seen.add(node);
    visit(node, path);
    for (const [key, value] of Object.entries(node)) {
      if (key.startsWith('x-') || key === 'examples' || key === 'example' || key === 'enum') continue;
      walk(value, `${path}/${key}`);
    }
  };
  walk(doc.components?.schemas, '#/components/schemas');
};

/** Every deprecated element of the contract: [{ kind, name, sunset, guide }] sorted by name. */
export function deprecations(doc) {
  const rows = [];
  for (const { kind, key, method, op } of operationsOf(doc)) {
    if (op.deprecated === true) {
      rows.push({ kind: kind === 'webhook' ? 'event' : 'operation', name: kind === 'webhook' ? key : `${method.toUpperCase()} ${key}`, sunset: op['x-orcon-sunset'], guide: op['x-orcon-deprecation-guide'] });
    }
    for (const p of op.parameters ?? []) {
      if (p.deprecated === true) rows.push({ kind: 'parameter', name: `${method.toUpperCase()} ${key} ${p.in} parameter \`${p.name}\``, sunset: p['x-orcon-sunset'], guide: p['x-orcon-deprecation-guide'] });
    }
  }
  walkSchemas(doc, (node, path) => {
    const m = /^#\/components\/schemas\/([^/]+)\/properties\/([^/]+)$/.exec(path);
    if (m && node.deprecated === true) rows.push({ kind: 'field', name: `\`${m[1]}.${m[2]}\``, sunset: node['x-orcon-sunset'], guide: node['x-orcon-deprecation-guide'] });
  });
  return rows.sort((a, b) => `${a.kind} ${a.name}`.localeCompare(`${b.kind} ${b.name}`));
}

/** The experimental operations and events of `head`, marking those absent from `base`. */
export function experimental(head, base) {
  const id = (o) => `${o.kind}:${o.method}:${o.key}`;
  const before = new Set(base ? operationsOf(base).filter((o) => o.op['x-orcon-stability'] === 'experimental').map(id) : []);
  return operationsOf(head)
    .filter((o) => o.op['x-orcon-stability'] === 'experimental')
    .map((o) => ({ name: o.kind === 'webhook' ? o.key : `${o.method.toUpperCase()} ${o.key}`, operationId: o.op.operationId ?? '', summary: o.op.summary ?? '', isNew: !before.has(id(o)) }))
    .sort((a, b) => a.name.localeCompare(b.name));
}

/** Markdown of the "Deprecations" section. */
export function renderDeprecations(rows) {
  if (rows.length === 0) return 'Nothing is deprecated.\n';
  return [
    '| Kind | Name | Sunset | Migration guide |',
    '|---|---|---|---|',
    ...rows.map((r) => `| ${r.kind} | ${cell(r.name)} | ${r.sunset ?? 'not set'} | ${r.guide ? `<${r.guide}>` : 'not set'} |`),
    '',
  ].join('\n');
}

/** Markdown of the "Experimental" section. */
export function renderExperimental(rows) {
  if (rows.length === 0) return 'There are no experimental operations.\n';
  return [
    'Experimental operations need the `Orcon-Experimental` header and can change or disappear in any release.',
    '',
    '| Operation | Operation id | Summary | Since the previous release |',
    '|---|---|---|---|',
    ...rows.map((r) => `| ${cell(r.name)} | \`${r.operationId}\` | ${cell(r.summary)} | ${r.isNew ? 'new' : 'existing'} |`),
    '',
  ].join('\n');
}

if (isMain(import.meta.url)) {
  const { flags } = parseArgs(process.argv.slice(2));
  must(flags.head, 'usage: changelog-deprecations.mjs --head <bundle> [--base <bundle>]');
  const head = readDocument(flags.head);
  const base = flags.base ? readDocument(flags.base) : undefined;
  process.stdout.write(`## Deprecations\n\n${renderDeprecations(deprecations(head))}\n## Experimental\n\n${renderExperimental(experimental(head, base))}`);
}
