// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { test } from 'node:test';

import { VERBS } from '../plugin/verbs.js';
import { repoRoot } from '../scripts/lib.mjs';

const paths = readFileSync(join(repoRoot(), '.agents', 'skills', 'orcon-api', 'references', 'paths.md'), 'utf8');

/** First column of the PATH-5 table: rows like "| `activate` | make a resource ... |". */
function tableVerbs() {
  const start = paths.indexOf('**PATH-5**');
  const end = paths.indexOf('[`action-verb`]', start);
  assert.ok(start >= 0 && end > start, 'PATH-5 and its table were not found in paths.md');
  return [...paths.slice(start, end).matchAll(/^\s*\|\s*`([a-z]+)`\s*\|/gm)].map((m) => m[1]);
}

test('verbs.js has 31 verbs and equals the PATH-5 table of paths.md', () => {
  const table = tableVerbs();
  assert.equal(table.length, 31);
  assert.equal(VERBS.length, 31);
  assert.deepEqual([...VERBS], table);
});

test('verbs.js contains the verbs introduced by the amendments and by Rev. 3', () => {
  for (const verb of ['evaluate', 'authorize', 'reindex']) assert.ok(VERBS.includes(verb), verb);
});

test('ADR 0080 lists the same verbs in the same order', () => {
  const adr = readFileSync(join(repoRoot(), 'docs', 'adr', '0080-public-api-design-rules-and-contract-toolchain.md'), 'utf8');
  const line = adr.split('\n').find((l) => l.startsWith('2. Paths'));
  const verbs = line.match(/thirty-one verbs: ([a-z, ]+)\./)[1].split(',').map((v) => v.trim());
  assert.deepEqual(verbs, [...VERBS]);
});
