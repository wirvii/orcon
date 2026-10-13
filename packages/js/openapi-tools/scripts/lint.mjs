// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Lints the contract with redocly.yaml and the orcon plugin.
 *
 *   node scripts/lint.mjs [--input <openapi.yaml>] [--config <redocly.yaml>]
 *
 * With no input it lints packages/openapi/v1/openapi.yaml, and reports (exit 0) that there
 * is nothing to lint while that file does not exist yet. Any finding is an error.
 */

import { existsSync } from 'node:fs';
import { join, resolve } from 'node:path';

import { defaultContract, packageDir, parseArgs, redocly, repoRoot } from './lib.mjs';

const { flags } = parseArgs(process.argv.slice(2));
const input = flags.input ? resolve(flags.input) : defaultContract();
const config = flags.config ? resolve(flags.config) : join(packageDir, 'redocly.yaml');

if (!existsSync(input)) {
  process.stdout.write(`api-lint: no contract at ${input}; nothing to lint\n`);
  process.exit(0);
}

const result = redocly(['lint', input, '--config', config, '--max-problems', '1000'], { cwd: repoRoot() });
process.stdout.write(result.stdout);
process.stderr.write(result.stderr);
process.exit(result.status);
