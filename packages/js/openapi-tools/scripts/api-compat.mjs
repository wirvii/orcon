// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * `make api-compat` (R-COMPAT-01, spec section 4.8): compares the contract of the working tree
 * with the contract of the last released `v*` tag.
 *
 *   node scripts/api-compat.mjs [--base <git ref>] [--oasdiff "<command>"] [--oasdiff-cwd <dir>]
 *        [--summary <git ref>]
 *
 * With no released tag, or no contract in the base, it says "no released contract" and exits 0.
 * `--summary <ref>` also prints the oasdiff changelog against that ref (a pull request base),
 * for the job summary; it never fails the command.
 */

import { appendFileSync } from 'node:fs';
import { resolve } from 'node:path';

import { contractExistsAt, bundledDocumentAt, bundledDocumentOfWorkingTree, latestReleaseTag } from './api-git.mjs';
import { composeChangelog } from './changelog.mjs';
import { compatVerdict } from './compat-selftest.mjs';
import { parseArgs } from './lib.mjs';

const { flags } = parseArgs(process.argv.slice(2));
const command = flags.oasdiff ?? process.env.ORCON_OASDIFF ?? 'oasdiff';
const cwd = flags['oasdiff-cwd'] ? resolve(flags['oasdiff-cwd']) : undefined;

const head = bundledDocumentOfWorkingTree();
if (!head) {
  process.stdout.write('api-compat: no contract in the working tree; nothing to compare\n');
  process.exit(0);
}

if (flags.summary && process.env.GITHUB_STEP_SUMMARY && contractExistsAt(flags.summary)) {
  try {
    const text = composeChangelog({ base: bundledDocumentAt(flags.summary), head, version: 'this pull request', from: flags.summary, command, cwd });
    appendFileSync(process.env.GITHUB_STEP_SUMMARY, `${text}\n`);
  } catch (error) {
    process.stdout.write(`api-compat: the informational changelog failed: ${error.message}\n`);
  }
}

const baseRef = flags.base ?? latestReleaseTag();
if (!baseRef || !contractExistsAt(baseRef)) {
  process.stdout.write('api-compat: no released contract; nothing to compare\n');
  process.exit(0);
}
process.stdout.write(`api-compat: comparing with ${baseRef}\n`);
const got = compatVerdict(command, bundledDocumentAt(baseRef), head, cwd);
process.stdout.write(got.output);
if (got.verdict === 'fail') {
  process.stdout.write(`api-compat: breaking changes on stable or beta since ${baseRef} (detectors: ${got.detectors.join(', ')})\n`);
  process.exit(1);
}
process.stdout.write('api-compat: no breaking change\n');
