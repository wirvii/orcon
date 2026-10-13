// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { readFileSync, readdirSync } from 'node:fs';
import { join } from 'node:path';
import { test } from 'node:test';

import { LEVELS_FILE, compatVerdict, expectation, splitCommand } from '../scripts/compat-selftest.mjs';
import { readDocument } from '../scripts/doc-io.mjs';
import { selftestChangelog } from '../scripts/changelog-selftest.mjs';
import { packageDir } from '../scripts/lib.mjs';
import { changelogPair } from './support/changelog-case.mjs';
import { CASES, buildCompatCases } from './support/compat-cases.mjs';
import { findOasdiff } from './support/oasdiff.mjs';

const root = join(packageDir, 'testdata', 'compat');
const oasdiff = findOasdiff();
const skip = oasdiff ? false : 'oasdiff is not available (set ORCON_OASDIFF, install oasdiff, or run after the make cut adds the tool)';
const levels = readFileSync(LEVELS_FILE, 'utf8').split('\n').filter(Boolean).map((l) => l.trim().split(/\s+/));

test('the compatibility pairs are the generated ones (run `node test/support/write-compat-cases.mjs`)', () => {
  const generated = buildCompatCases();
  assert.deepEqual(readdirSync(root).sort(), [...generated.keys()].sort());
  for (const [name, files] of generated) {
    for (const [kind, text] of Object.entries(files)) {
      const current = readFileSync(join(root, name, `${kind}.yaml`), 'utf8');
      assert.ok(current.endsWith(text), `${name}/${kind}.yaml differs from its generator`);
    }
  }
});

test('the changelog pair is the generated one', () => {
  const pair = changelogPair();
  for (const kind of ['base', 'head']) {
    assert.ok(readFileSync(join(packageDir, 'testdata', 'changelog', `${kind}.yaml`), 'utf8').endsWith(pair[kind]), `${kind}.yaml differs from its generator`);
  }
});

test('there is a pair for each normative row of VER-2 and VER-3 the toolchain covers', () => {
  const rows = Object.values(CASES).map((c) => c.row).join(' | ');
  for (const needle of ['removing an operation', 'removing a response field', 'removing a response enum value', 'adding a required request parameter', 'making an optional request field required', 'changing a field type', 'narrowing accepted values', 'stricter pagination', 'a stricter permission', 'stricter idempotency', 'changing the default sort', 'changing the error code for the same condition', 'new optional request field', 'new response field', 'new response enum value', 'new operation', 'webhooks: removing an event type', 'webhooks: removing a required event payload field', 'webhooks: new required event payload field']) {
    assert.ok(rows.includes(needle), `no pair for: ${needle}`);
  }
});

test('the severity file has valid lines and a pair for every check it raises or lowers', () => {
  assert.ok(levels.length > 0);
  const covered = new Set(Object.values(CASES).map((c) => c.check).filter(Boolean));
  for (const [id, level, ...rest] of levels) {
    assert.deepEqual(rest, [], `${id}: one check id and one level per line, no comments`);
    assert.ok(['err', 'info'].includes(level), `${id}: level ${level}`);
    assert.ok(covered.has(id), `no compatibility pair exercises ${id}`);
  }
});

test('the expectation of each pair names a detector for a failure', () => {
  for (const name of readdirSync(root)) {
    const e = expectation(join(root, name, 'expect.yaml'));
    assert.ok(['fail', 'pass'].includes(e.expect), name);
    if (e.expect === 'fail') assert.ok(['oasdiff', 'extra'].includes(e.detector), name);
  }
});

test('every pair gets the expected verdict and detector', { skip }, () => {
  const mismatches = [];
  for (const name of readdirSync(root)) {
    const e = expectation(join(root, name, 'expect.yaml'));
    const got = compatVerdict(oasdiff.command, readDocument(join(root, name, 'base.yaml')), readDocument(join(root, name, 'head.yaml')), oasdiff.cwd);
    if (got.verdict !== e.expect || (e.expect === 'fail' && !got.detectors.includes(e.detector))) mismatches.push(`${name}: expected ${e.expect} by ${e.detector}, got ${got.verdict} by ${got.detectors}`);
  }
  assert.deepEqual(mismatches, []);
});

test('the pair of each check in the severity file makes oasdiff report that check', { skip }, () => {
  const [program, ...prefix] = splitCommand(oasdiff.command);
  for (const [id] of levels) {
    const name = Object.keys(CASES).find((n) => CASES[n].check === id);
    const result = spawnSync(program, [...prefix, 'changelog', '--format', 'json', join(root, name, 'base.yaml'), join(root, name, 'head.yaml')], { encoding: 'utf8', cwd: oasdiff.cwd, env: { ...process.env, GOWORK: 'off' } });
    assert.ok(JSON.parse(result.stdout).some((c) => c.id === id), `${name} does not trigger ${id}`);
  }
});

test('the changelog of the fixed pair equals testdata/changelog/expected.md', { skip }, () => {
  const expected = readFileSync(join(packageDir, 'testdata', 'changelog', 'expected.md'), 'utf8');
  assert.equal(selftestChangelog(oasdiff.command, oasdiff.cwd), expected);
});
