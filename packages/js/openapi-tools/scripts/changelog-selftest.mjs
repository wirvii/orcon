// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Self-test of the changelog (T10): generates the changelog of the fixed pair in
 * testdata/changelog and compares it with testdata/changelog/expected.md.
 *
 *   node scripts/changelog-selftest.mjs [--oasdiff "<command>"] [--oasdiff-cwd <dir>] [--update]
 *
 * --update rewrites expected.md; a person reviews the diff before committing it.
 */

import { existsSync, readFileSync, writeFileSync } from 'node:fs';
import { join, resolve } from 'node:path';

import { composeChangelog } from './changelog.mjs';
import { must, readDocument } from './doc-io.mjs';
import { isMain, packageDir, parseArgs } from './lib.mjs';

export const SELFTEST = { version: 'v1.1.0', from: 'v1.0.0' };

/** Generates the changelog of the fixed pair. */
export function selftestChangelog(command, cwd) {
  const dir = join(packageDir, 'testdata', 'changelog');
  return composeChangelog({
    base: readDocument(join(dir, 'base.yaml')),
    head: readDocument(join(dir, 'head.yaml')),
    version: SELFTEST.version,
    from: SELFTEST.from,
    command,
    cwd,
  });
}

if (isMain(import.meta.url)) {
  const { flags } = parseArgs(process.argv.slice(2), ['update']);
  const command = flags.oasdiff ?? process.env.ORCON_OASDIFF ?? 'oasdiff';
  const cwd = flags['oasdiff-cwd'] ? resolve(flags['oasdiff-cwd']) : undefined;
  const expectedFile = join(packageDir, 'testdata', 'changelog', 'expected.md');
  const text = selftestChangelog(command, cwd);
  if (flags.update) {
    writeFileSync(expectedFile, text);
    process.stdout.write(`api-changelog-selftest: wrote ${expectedFile}\n`);
  } else {
    must(existsSync(expectedFile), `${expectedFile} does not exist; run with --update`);
    const expected = readFileSync(expectedFile, 'utf8');
    must(text === expected, `api-changelog-selftest: the generated changelog differs from testdata/changelog/expected.md\n--- generated ---\n${text}`);
    process.stdout.write('api-changelog-selftest: ok\n');
  }
}
