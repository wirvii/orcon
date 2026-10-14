// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * `make api-changelog` and `make api-changelog-check` (R-CHG-01, spec section 4.8): writes
 * (or, with --check, verifies) docs/reference/api/changelog/<version>.md for the contract of
 * the working tree against the last released tag (or --from).
 *
 *   node scripts/api-changelog.mjs --version vX.Y.Z [--from <tag>] [--check]
 *        [--oasdiff "<command>"] [--oasdiff-cwd <dir>]
 *
 * With no contract in the working tree it says so and exits 0 (nothing to release yet). With
 * no earlier tag the base is an empty contract: everything is new.
 */

import { existsSync, readFileSync } from 'node:fs';
import { join, resolve } from 'node:path';

import { bundledDocumentAt, bundledDocumentOfWorkingTree, contractExistsAt, latestReleaseTag } from './api-git.mjs';
import { composeChangelog } from './changelog.mjs';
import { must, writeText } from './doc-io.mjs';
import { parseArgs, repoRoot } from './lib.mjs';

const { flags } = parseArgs(process.argv.slice(2), ['check']);
must(/^v\d+\.\d+\.\d+/.test(flags.version ?? ''), 'usage: api-changelog.mjs --version vX.Y.Z [--from <tag>] [--check]');
const command = flags.oasdiff ?? process.env.ORCON_OASDIFF ?? 'oasdiff';
const cwd = flags['oasdiff-cwd'] ? resolve(flags['oasdiff-cwd']) : undefined;

const head = bundledDocumentOfWorkingTree();
if (!head) {
  process.stdout.write('api-changelog: no contract in the working tree; nothing to write\n');
  process.exit(0);
}

const from = flags.from ?? latestReleaseTag('HEAD', flags.version);
let base = { openapi: '3.1.0', info: { title: 'empty', version: '0.0.0' }, paths: {} };
let label = 'the start of the API';
if (from && contractExistsAt(from)) {
  base = bundledDocumentAt(from);
  label = from;
}
const text = composeChangelog({ base, head, version: flags.version, from: label, command, cwd });
const file = join(repoRoot(), 'docs', 'reference', 'api', 'changelog', `${flags.version}.md`);
if (flags.check) {
  const current = existsSync(file) ? readFileSync(file, 'utf8') : '';
  must(current === text, `${file} differs from the generated changelog; run make api-changelog VERSION=${flags.version} and commit it`);
  process.stdout.write(`api-changelog-check: ${flags.version} ok\n`);
} else {
  writeText(file, text);
  process.stdout.write(`api-changelog: wrote ${file} (compared with ${label})\n`);
}
