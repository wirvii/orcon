// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import assert from 'node:assert/strict';
import { existsSync, readFileSync, readdirSync } from 'node:fs';
import { join } from 'node:path';
import { test } from 'node:test';

import { RULE_NAMES } from '../plugin/orcon.js';
import { packageDir, repoRoot } from '../scripts/lib.mjs';
import { CASES, buildBadCases } from './support/bad-cases.mjs';
import { lint, orconRules } from './support/lint.mjs';

const badDir = join(packageDir, 'testdata', 'bad');
const generated = buildBadCases();

test('there are 31 planted cases and each orcon rule has one', () => {
  assert.equal(RULE_NAMES.length, 31);
  assert.equal(generated.size, 31);
  assert.deepEqual([...new Set(Object.values(CASES).map((c) => c.rule))].sort(), [...RULE_NAMES].sort());
  assert.ok(existsSync(join(badDir, 'response-envelope.yaml')), 'response-envelope.yaml is the planted case of orcon/response-shape');
});

test('the planted case files are the generated ones (run `pnpm --filter @orcon/openapi-tools gen:bad`)', () => {
  assert.deepEqual(readdirSync(badDir).sort(), [...generated.keys()].map((n) => `${n}.yaml`).sort());
  for (const [name, text] of generated) {
    assert.equal(readFileSync(join(badDir, `${name}.yaml`), 'utf8'), text, `${name}.yaml differs from its generator`);
  }
});

test('the planted case list equals the one in review.md', () => {
  const review = readFileSync(join(repoRoot(), '.agents', 'skills', 'orcon-api', 'references', 'review.md'), 'utf8');
  const section = review.slice(review.indexOf('## Planted cases'), review.indexOf('## Inherited checks'));
  const listed = [...section.matchAll(/`([a-z-]+)\.yaml`/g)].map((m) => m[1]).filter((n) => n !== 'openapi');
  assert.deepEqual(listed.sort(), Object.keys(CASES).sort());
});

for (const [name, spec] of Object.entries(CASES)) {
  test(`planted case ${name} fails with exactly orcon/${spec.rule}`, () => {
    const problems = lint(join(badDir, `${name}.yaml`));
    assert.deepEqual(orconRules(problems), [`orcon/${spec.rule}`], problems.map((p) => `${p.ruleId}: ${p.message}`).join('\n'));
  });
}
