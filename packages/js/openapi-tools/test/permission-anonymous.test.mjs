// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * The anonymous permission of orcon/permission (ADR-0080 section 4, as amended by SPEC-016): it is valid
 * on protocol-tagged operations and on the closed list of sign-in and setup operations, and nowhere else.
 * Each case starts from the bundled good document, applies one change and lints it.
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

const ANONYMOUS_MESSAGE = /anonymous is allowed only on protocol operations/;
const FLOW_GET = '/flows/{flowId}';

let work;
let base;
let counter = 0;

before(() => {
  work = mkdtempSync(join(tmpdir(), 'orcon-anonymous-'));
  const out = join(work, 'good.yaml');
  bundle(join(packageDir, 'testdata', 'good', 'openapi.yaml'), out);
  base = readDocument(out);
});

after(() => rmSync(work, { recursive: true, force: true }));

/** Lints a mutated copy of the good document and returns the orcon/permission problems. */
function permissionProblems(mutate) {
  const doc = structuredClone(base);
  mutate(doc);
  const file = join(work, `case-${(counter += 1)}.yaml`);
  writeFileSync(file, stringify(doc, { lineWidth: 0 }));
  return lint(file).filter((p) => p.ruleId === 'orcon/permission');
}

/** Gives the flow read operation the given operationId and the anonymous permission. */
const anonymousFlowRead = (operationId) => (d) => {
  d.paths[FLOW_GET].get.operationId = operationId;
  d.paths[FLOW_GET].get['x-orcon-permission'] = 'anonymous';
};

test('permission accepts anonymous on a protocol operation', () => {
  const op = base.paths['/oauth/token'].post;
  assert.ok(op.tags.includes('protocol'), 'the fixture operation is protocol-tagged');
  assert.equal(op['x-orcon-permission'], 'anonymous');
  assert.deepEqual(permissionProblems(() => {}), []);
});

test('permission accepts anonymous on an allowlisted operation', () => {
  for (const operationId of ['createSession', 'getSetupState', 'createSetupSession']) {
    assert.deepEqual(permissionProblems(anonymousFlowRead(operationId)), [], operationId);
  }
});

test('permission rejects anonymous on any other operation', () => {
  for (const operationId of ['getFlow', 'createSessionV2', 'createsession', 'getSetupStateAll']) {
    const problems = permissionProblems(anonymousFlowRead(operationId));
    assert.equal(problems.length, 1, `${operationId}: ${JSON.stringify(problems)}`);
    assert.match(problems[0].message, ANONYMOUS_MESSAGE, operationId);
  }
});
