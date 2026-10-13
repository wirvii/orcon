// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Generates docs/reference/api/README.md, the operation index of the bundled contract.
 *
 *   node scripts/operation-index.mjs --input <bundle> --output <README.md> [--check]
 *
 * With --check nothing is written; the exit status is 1 when the file differs.
 */

import { existsSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';

import { cell, formatPermission, generatedHeader, must, operationsOf, readDocument, writeText } from './doc-io.mjs';
import { isMain, parseArgs } from './lib.mjs';

const cli = (op) => {
  const value = op['x-orcon-cli'];
  if (value?.command) return `\`orcon ${value.command}\``;
  if (value?.none) return `none (${value.none.category})`;
  return '';
};

const mcp = (op) => {
  const value = op['x-orcon-mcp'];
  if (!value || value.level === 'none') return 'none';
  return `\`${value.tool}\` (${value.level})`;
};

/** Renders the operation index for a bundled document. */
export function renderOperationIndex(doc) {
  const all = operationsOf(doc);
  const paths = all
    .filter((o) => o.kind === 'path')
    .sort((a, b) => (a.key === b.key ? a.method.localeCompare(b.method) : a.key.localeCompare(b.key)));
  const webhooks = all.filter((o) => o.kind === 'webhook').sort((a, b) => a.key.localeCompare(b.key));
  const lines = [
    generatedHeader('scripts/operation-index.mjs of @orcon/openapi-tools (make api-lint)'),
    '# API operation index',
    '',
    `Generated from the API contract: ${paths.length} operations and ${webhooks.length} event types. The reference site at \`/api/reference\` has the full description of each operation.`,
    '',
    '## Operations',
    '',
    '| Method | Path | Operation id | Summary | Stability | Permission | CLI | MCP |',
    '|---|---|---|---|---|---|---|---|',
    ...paths.map(
      ({ key, method, op }) =>
        `| ${method.toUpperCase()} | \`/api/v1${key}\` | \`${op.operationId}\` | ${cell(op.summary)} | ${op['x-orcon-stability'] ?? ''} | ${formatPermission(op['x-orcon-permission'])} | ${cli(op)} | ${mcp(op)} |`,
    ),
    '',
    '## Events',
    '',
    '| Event type | CloudEvents type | Permission | Stability | Since |',
    '|---|---|---|---|---|',
    ...webhooks.map(({ key, op }) => {
      const e = op['x-orcon-event'] ?? {};
      return `| \`${key}\` | \`${e.cloudEventsType ?? ''}\` | \`${e.permission ?? ''}\` | ${e.stability ?? ''} | ${e.since ?? ''} |`;
    }),
    '',
  ];
  return lines.join('\n');
}

if (isMain(import.meta.url)) {
  const { flags } = parseArgs(process.argv.slice(2), ['check']);
  must(flags.input && flags.output, 'usage: operation-index.mjs --input <bundle> --output <README.md> [--check]');
  const text = renderOperationIndex(readDocument(flags.input));
  if (flags.check) {
    const current = existsSync(resolve(flags.output)) ? readFileSync(resolve(flags.output), 'utf8') : '';
    must(current === text, `${flags.output} is out of date; run make api-lint (or scripts/operation-index.mjs) and commit the result`);
  } else {
    writeText(flags.output, text);
  }
}
