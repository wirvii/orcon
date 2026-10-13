// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import assert from 'node:assert/strict';
import { join } from 'node:path';
import { test } from 'node:test';

import { packageDir } from '../scripts/lib.mjs';
import { lint } from './support/lint.mjs';

test('the good document has no problem (built-in rules and all 31 orcon rules)', () => {
  const problems = lint(join(packageDir, 'testdata', 'good', 'openapi.yaml'));
  assert.deepEqual(problems, []);
});
