// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Generates docs/reference/problem-codes.md from the problem code catalog of the bundled
 * contract (components/schemas/ProblemCode, x-orcon-problem-catalog; ERR-6).
 *
 *   node scripts/problem-codes.mjs --input <bundle> --output <problem-codes.md> [--check]
 *
 * With --check nothing is written; the exit status is 1 when the file differs.
 */

import { existsSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';

import { cell, generatedHeader, must, problemCatalog, readDocument, writeText } from './doc-io.mjs';
import { isMain, parseArgs } from './lib.mjs';

/** Renders the problem code page for a bundled document. */
export function renderProblemCodes(doc) {
  const catalog = problemCatalog(doc);
  const rows = [...catalog.entries()].sort((a, b) => a[1].status - b[1].status);
  const lines = [
    generatedHeader('scripts/problem-codes.mjs of @orcon/openapi-tools (make api-lint)'),
    '# Problem codes',
    '',
    'Every error of the public API is an `application/problem+json` document (RFC 9457). Its `type` is `urn:orcon:problem:<code>` and its `code` repeats `<code>`; codes are stable. The catalog below is the one declared in the API contract (`components/schemas/ProblemCode`).',
    '',
    '| Code | Status | Title | When | Retryable |',
    '|---|---|---|---|---|',
    ...rows.map(([code, info]) => `| <a id="${code}"></a>\`${code}\` | ${info.status} | ${cell(info.title)} | ${cell(info.when)} | ${info.retryable ? 'yes' : 'no'} |`),
    '',
  ];
  return lines.join('\n');
}

if (isMain(import.meta.url)) {
  const { flags } = parseArgs(process.argv.slice(2), ['check']);
  must(flags.input && flags.output, 'usage: problem-codes.mjs --input <bundle> --output <problem-codes.md> [--check]');
  const text = renderProblemCodes(readDocument(flags.input));
  if (flags.check) {
    const current = existsSync(resolve(flags.output)) ? readFileSync(resolve(flags.output), 'utf8') : '';
    must(current === text, `${flags.output} is out of date; run make api-lint (or scripts/problem-codes.mjs) and commit the result`);
  } else {
    writeText(flags.output, text);
  }
}
