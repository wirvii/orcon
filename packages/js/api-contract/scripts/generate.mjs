// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Generates the Zod schemas, example data and Mock Service Worker handlers from the OpenAPI
// document with Kubb (kubb.config.ts), then writes the licence header into the barrel files
// Kubb leaves bare.
//
//   node scripts/generate.mjs                        the real contract (packages/openapi/v1) into src/gen
//   node scripts/generate.mjs --input a.yaml --output dir
//
// Until the real document exists (SPEC-005 API1) the default run prints a SKIP line and exits 0.

import { execFileSync } from 'node:child_process';
import { existsSync, readdirSync, readFileSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const defaultInput = resolve(root, '../../openapi/v1/openapi.yaml');
const header = [
  '// SPDX-FileCopyrightText: The Orcon Authors',
  '// SPDX-License-Identifier: Apache-2.0',
  '//',
  '// Generated from the OpenAPI document by Kubb. Do not edit; run `pnpm generate`.',
  '',
  '',
].join('\n');

function option(name) {
  const at = process.argv.indexOf(`--${name}`);
  return at === -1 ? undefined : process.argv[at + 1];
}

const explicitInput = option('input');
const input = explicitInput ? resolve(process.cwd(), explicitInput) : defaultInput;
const output = resolve(process.cwd(), option('output') ?? join(root, 'src/gen'));

if (!existsSync(input)) {
  if (explicitInput) {
    console.error(`generate: the OpenAPI document ${input} does not exist`);
    process.exit(1);
  }
  console.log('SKIP: OpenAPI document not present (SPEC-005 API1)');
  process.exit(0);
}

execFileSync(join(root, 'node_modules/.bin/kubb'), ['generate', '--config', 'kubb.config.ts'], {
  cwd: root,
  stdio: ['ignore', 'ignore', 'inherit'],
  env: { ...process.env, ORCON_CONTRACT_INPUT: input, ORCON_CONTRACT_OUTPUT: output },
});

/** Prepends the header to every barrel (index.ts) that does not start with one. */
function addHeaders(directory) {
  for (const entry of readdirSync(directory, { withFileTypes: true })) {
    const path = join(directory, entry.name);
    if (entry.isDirectory()) {
      addHeaders(path);
    } else if (entry.name === 'index.ts') {
      const text = readFileSync(path, 'utf8');
      if (!text.startsWith('// SPDX-FileCopyrightText')) writeFileSync(path, header + text);
    }
  }
}
addHeaders(output);
console.log(`generate: wrote the contract code to ${output}`);
