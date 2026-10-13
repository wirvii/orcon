// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import assert from 'node:assert/strict';
import { cpSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { test } from 'node:test';

import { packageDir } from '../scripts/lib.mjs';
import { lint, orconRules } from './support/lint.mjs';

test('enum-upper-snake also checks a document schema the contract references in another file', () => {
  const dir = mkdtempSync(join(tmpdir(), 'orcon-external-doc-'));
  try {
    cpSync(join(packageDir, 'testdata', 'good'), dir, { recursive: true });
    const doc = join(dir, 'schemas', 'flow-document.yaml');
    writeFileSync(doc, readFileSync(doc, 'utf8').replace('[TRIGGER, ACTION, WAIT_FOR_HUMAN]', '[TRIGGER, action, WAIT_FOR_HUMAN]'));
    const problems = lint(join(dir, 'openapi.yaml'));
    assert.deepEqual(orconRules(problems), ['orcon/enum-upper-snake']);
    assert.ok(problems.some((p) => p.file?.endsWith('flow-document.yaml')), 'the finding points into the referenced document');
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});
