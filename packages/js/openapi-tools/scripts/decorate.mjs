// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Adds the generated sections to the descriptions of the PUBLISHED copy of the contract
 * (DOC-3, spec section 4.7): required permission, rate class, stability, idempotency,
 * problem codes, CLI command and MCP tool. The contract itself never contains them.
 *
 *   node scripts/decorate.mjs --input <bundle> --output <decorated.json>
 */

import { cell, formatPermission, must, operationsOf, problemCatalog, readDocument, writeJson } from './doc-io.mjs';
import { isMain, parseArgs } from './lib.mjs';

/** Anchor of the problem code table in the description of the reference site. */
export const PROBLEM_CODES_ANCHOR = '#description/problem-codes';

function section(lines) {
  return `\n\n---\n\n${lines.join('\n')}\n`;
}

function operationSection(op) {
  const lines = ['**Access and behavior**', ''];
  lines.push(`- Required permission: ${formatPermission(op['x-orcon-permission'])}`);
  if (op['x-orcon-rate-class']) lines.push(`- Rate class: \`${op['x-orcon-rate-class']}\``);
  if (op['x-orcon-stability']) lines.push(`- Stability: \`${op['x-orcon-stability']}\``);
  if (op['x-orcon-idempotency']) lines.push(`- Idempotency: \`${op['x-orcon-idempotency']}\` (header \`Idempotency-Key\`)`);
  if (op['x-orcon-long-running']) lines.push(`- Long-running: yes (\`${op['x-orcon-long-running'].kind}\`); follow the \`Location\` header`);
  if (Array.isArray(op['x-orcon-problems']) && op['x-orcon-problems'].length > 0) {
    lines.push(`- Problem codes: ${op['x-orcon-problems'].map((c) => `[\`${c}\`](${PROBLEM_CODES_ANCHOR})`).join(', ')}`);
  }
  const cli = op['x-orcon-cli'];
  if (cli?.command) lines.push(`- CLI: \`orcon ${cli.command}\``);
  else if (cli?.none) lines.push(`- CLI: not available (${cli.none.category}: ${cli.none.reason})`);
  const mcp = op['x-orcon-mcp'];
  if (mcp && mcp.level !== 'none') {
    lines.push(`- MCP: tool \`${mcp.tool}\`, level \`${mcp.level}\`${mcp.level === 'destructive' ? ' (a person confirms each call)' : ''}`);
  } else if (mcp) {
    lines.push('- MCP: not exposed');
  }
  if (op.deprecated) {
    lines.push(`- Deprecated: sunset \`${op['x-orcon-sunset'] ?? 'not set'}\`${op['x-orcon-deprecation-guide'] ? `, [migration guide](${op['x-orcon-deprecation-guide']})` : ''}`);
  }
  return section(lines);
}

function eventSection(op) {
  const e = op['x-orcon-event'] ?? {};
  return section([
    '**Event**',
    '',
    `- Event type: \`${e.type}\``,
    `- CloudEvents type: \`${e.cloudEventsType}\``,
    `- Required permission to receive it: \`${e.permission}\``,
    `- Stability: \`${e.stability}\``,
    `- Since: \`${e.since}\``,
  ]);
}

function problemTable(doc) {
  const catalog = problemCatalog(doc);
  if (catalog.size === 0) return '';
  const rows = [...catalog.entries()].sort((a, b) => a[1].status - b[1].status);
  return [
    '',
    '',
    '## Problem codes',
    '',
    'Errors are `application/problem+json` documents whose `type` is `urn:orcon:problem:<code>`.',
    '',
    '| Code | Status | When | Retryable |',
    '|---|---|---|---|',
    ...rows.map(([code, info]) => `| \`${code}\` | ${info.status} | ${cell(info.when)} | ${info.retryable ? 'yes' : 'no'} |`),
    '',
  ].join('\n');
}

/** Returns a decorated deep copy of a bundled document. */
export function decorate(doc) {
  const out = structuredClone(doc);
  for (const { kind, op } of operationsOf(out)) {
    op.description = `${(op.description ?? '').trimEnd()}${kind === 'webhook' ? eventSection(op) : operationSection(op)}`;
  }
  out.info = out.info ?? {};
  out.info.description = `${(out.info.description ?? '').trimEnd()}${problemTable(doc)}`;
  return out;
}

if (isMain(import.meta.url)) {
  const { flags } = parseArgs(process.argv.slice(2));
  must(flags.input && flags.output, 'usage: decorate.mjs --input <bundle> --output <decorated.json>');
  writeJson(flags.output, decorate(readDocument(flags.input)));
}
