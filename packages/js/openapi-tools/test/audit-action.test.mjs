// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * The audit action of orcon/audit (SPEC-004 Annex B, SPEC-016 R-SH-SES-06): an action is <area>.<event>, except the
 * single-word actions of docs/reference/audit-actions.md, of which logout is the one the contract uses.
 * Each case starts from the bundled good document, sets one operation's x-orcon-audit and lints it.
 */

import assert from 'node:assert/strict';
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { after, before, test } from 'node:test';

import { stringify } from 'yaml';

import { bundle } from '../scripts/bundle.mjs';
import { readDocument } from '../scripts/doc-io.mjs';
import { packageDir } from '../scripts/lib.mjs';
import { lint } from './support/lint.mjs';

const FLOW_GET = '/flows/{flowId}';

let work;
let base;
let counter = 0;

before(() => {
  work = mkdtempSync(join(tmpdir(), 'orcon-audit-'));
  const out = join(work, 'good.yaml');
  bundle(join(packageDir, 'testdata', 'good', 'openapi.yaml'), out);
  base = readDocument(out);
});

after(() => rmSync(work, { recursive: true, force: true }));

/** Lints a copy of the good document whose flow read declares the given audit action. */
function auditProblems(action) {
  const doc = structuredClone(base);
  doc.paths[FLOW_GET].get['x-orcon-audit'] = action;
  const file = join(work, `case-${(counter += 1)}.yaml`);
  writeFileSync(file, stringify(doc, { lineWidth: 0 }));
  return lint(file).filter((p) => p.ruleId === 'orcon/audit');
}

test('audit accepts an area.event action and the single-word logout', () => {
  for (const action of ['flow.published', 'session.revoked', 'login.succeeded', 'logout']) {
    assert.deepEqual(auditProblems(action), [], action);
  }
});

test('audit rejects other single-word and malformed actions', () => {
  for (const action of ['signout', 'Logout', 'logouts', 'log-out', 'flow', 'flow.Published']) {
    assert.equal(auditProblems(action).length, 1, action);
  }
});
