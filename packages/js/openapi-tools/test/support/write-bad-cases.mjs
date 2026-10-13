// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Regenerates testdata/bad/<case>.yaml from the good document.
 *
 *   node test/support/write-bad-cases.mjs
 */

import { mkdirSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';

import { packageDir } from '../../scripts/lib.mjs';
import { buildBadCases } from './bad-cases.mjs';

const dir = join(packageDir, 'testdata', 'bad');
mkdirSync(dir, { recursive: true });
const cases = buildBadCases();
for (const [name, text] of cases) writeFileSync(join(dir, `${name}.yaml`), text);
process.stdout.write(`wrote ${cases.size} planted cases to ${dir}\n`);
