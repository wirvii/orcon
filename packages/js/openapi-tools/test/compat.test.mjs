// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import assert from 'node:assert/strict';
import { readFileSync, readdirSync } from 'node:fs';
import { join } from 'node:path';
import { test } from 'node:test';

import { expectation, verdict } from '../scripts/compat-selftest.mjs';
import { packageDir } from '../scripts/lib.mjs';
import { selftestChangelog } from '../scripts/changelog-selftest.mjs';
import { changelogPair } from './support/changelog-case.mjs';
import { CASES, buildCompatCases } from './support/compat-cases.mjs';
import { findOasdiff } from './support/oasdiff.mjs';

const root = join(packageDir, 'testdata', 'compat');
const oasdiff = findOasdiff();
const skip = oasdiff ? false : 'oasdiff is not available (set ORCON_OASDIFF, install oasdiff, or run after the make cut adds the tool)';

test('the compatibility pairs are the generated ones (run `node test/support/write-compat-cases.mjs`)', () => {
  const generated = buildCompatCases();
  assert.deepEqual(readdirSync(root).sort(), [...generated.keys()].sort());
  for (const [name, files] of generated) {
    for (const [kind, text] of Object.entries(files)) {
      const file = join(root, name, `${kind}.yaml`);
      const current = readFileSync(file, 'utf8');
      assert.ok(current.endsWith(text), `${file} differs from its generator`);
    }
  }
});

test('the changelog pair is the generated one', () => {
  const pair = changelogPair();
  for (const kind of ['base', 'head']) {
    assert.ok(readFileSync(join(packageDir, 'testdata', 'changelog', `${kind}.yaml`), 'utf8').endsWith(pair[kind]), `${kind}.yaml differs from its generator`);
  }
});

test('there is a pair for each normative breaking and non-breaking row it covers', () => {
  const rows = Object.values(CASES).map((c) => c.row).join(' | ');
  for (const needle of ['removing an operation', 'removing a response field', 'removing a response enum value', 'adding a required request parameter', 'making an optional request field required', 'changing a field type', 'narrowing accepted values', 'stricter pagination', 'new optional request field', 'new response field', 'new operation', 'webhooks: removing an event type']) {
    assert.ok(rows.includes(needle), `no pair for: ${needle}`);
  }
});

for (const level of ['ERR', 'WARN']) {
  test(`every pair gets the expected verdict from oasdiff (--fail-on ${level})`, { skip }, () => {
    const mismatches = [];
    for (const name of readdirSync(root)) {
      const expected = expectation(join(root, name, 'expect.yaml'), level);
      const got = verdict(oasdiff.command, join(root, name, 'base.yaml'), join(root, name, 'head.yaml'), oasdiff.cwd, level);
      if (got.verdict !== (expected === 'gap' ? 'pass' : expected)) mismatches.push(`${name}: expected ${expected}, oasdiff says ${got.verdict}`);
    }
    assert.deepEqual(mismatches, []);
  });
}

test('the changelog of the fixed pair equals testdata/changelog/expected.md', { skip }, () => {
  const expected = readFileSync(join(packageDir, 'testdata', 'changelog', 'expected.md'), 'utf8');
  assert.equal(selftestChangelog(oasdiff.command, oasdiff.cwd), expected);
});
