// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { join } from 'node:path';
import { test } from 'node:test';

import { repoRoot } from '../scripts/lib.mjs';

test('the orcon-api templates compose into a document without findings', () => {
  const script = join(repoRoot(), '.agents', 'skills', 'orcon-api', 'scripts', 'check-api-templates.sh');
  const result = spawnSync('sh', [script], { encoding: 'utf8', cwd: repoRoot() });
  assert.equal(result.status, 0, `${result.stdout}\n${result.stderr}`);
  assert.match(result.stdout, /check-api-templates: ok/);
});
