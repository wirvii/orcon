// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Self-test of the compatibility check (R-COMPAT-01, ADR-0054 section 3): runs the same
 * command as `make api-compat` on every pair in testdata/compat and compares the verdict
 * with the expectation recorded next to the pair.
 *
 *   node scripts/compat-selftest.mjs [--oasdiff "<command>"] [--oasdiff-cwd <dir>] [--cases <dir>] [--fail-on ERR|WARN]
 *
 * The oasdiff command defaults to $ORCON_OASDIFF, then to `oasdiff` on PATH. The make target
 * passes `go tool oasdiff` from the tools module. Experimental operations are removed from
 * both sides first (scripts/stability-filter.mjs), exactly as in api-compat.
 */

import { spawnSync } from 'node:child_process';
import { mkdtempSync, readFileSync, readdirSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';

import { readDocument, writeYaml } from './doc-io.mjs';
import { isMain, packageDir, parseArgs } from './lib.mjs';
import { withoutExperimental } from './stability-filter.mjs';

/** The expected verdict of a pair for a fail-on level: the line `err:` or `warn:` of its expect.yaml file. */
export function expectation(file, level) {
  const line = readFileSync(file, 'utf8').split('\n').find((l) => l.startsWith(`${level.toLowerCase()}:`));
  if (!line) throw new Error(`${file} has no ${level.toLowerCase()}: line`);
  return line.split(':')[1].trim();
}

/** Splits a command string into [program, ...args] on whitespace. */
export const splitCommand = (command) => command.trim().split(/\s+/);

/** Runs `oasdiff breaking --fail-on ERR base head`; returns { status, output }. */
export function oasdiffBreaking(command, base, head, cwd, level = 'ERR') {
  const [program, ...prefix] = splitCommand(command);
  const result = spawnSync(program, [...prefix, 'breaking', '--fail-on', level, base, head], { encoding: 'utf8', cwd, env: { ...process.env, GOWORK: 'off' } });
  if (result.error) throw new Error(`cannot run ${command}: ${result.error.message}`);
  return { status: result.status ?? 1, output: `${result.stdout}${result.stderr}` };
}

/** Verdict of one pair after the stability filter: 'fail' (breaking) or 'pass'. */
export function verdict(command, baseFile, headFile, cwd, level = 'ERR') {
  const dir = mkdtempSync(join(tmpdir(), 'orcon-compat-'));
  try {
    const base = join(dir, 'base.yaml');
    const head = join(dir, 'head.yaml');
    writeYaml(base, withoutExperimental(readDocument(baseFile)));
    writeYaml(head, withoutExperimental(readDocument(headFile)));
    const { status, output } = oasdiffBreaking(command, base, head, cwd, level);
    if (status > 1) throw new Error(`oasdiff failed on ${baseFile}:\n${output}`);
    return { verdict: status === 1 ? 'fail' : 'pass', output };
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
}

if (isMain(import.meta.url)) {
  const { flags } = parseArgs(process.argv.slice(2));
  const command = flags.oasdiff ?? process.env.ORCON_OASDIFF ?? 'oasdiff';
  const root = resolve(flags.cases ?? join(packageDir, 'testdata', 'compat'));
  const cwd = flags['oasdiff-cwd'] ? resolve(flags['oasdiff-cwd']) : undefined;
  const level = flags['fail-on'] ?? 'ERR';
  const gaps = [];
  let failures = 0;
  const names = readdirSync(root, { withFileTypes: true }).filter((e) => e.isDirectory()).map((e) => e.name).sort();
  for (const name of names) {
    const expected = expectation(join(root, name, 'expect.yaml'), level);
    const got = verdict(command, join(root, name, 'base.yaml'), join(root, name, 'head.yaml'), cwd, level);
    // A documented gap is a breaking change per VER-2 that oasdiff does not report.
    const ok = got.verdict === (expected === 'gap' ? 'pass' : expected);
    if (!ok) failures += 1;
    if (expected === 'gap') gaps.push(name);
    process.stdout.write(`${ok ? 'ok  ' : 'FAIL'} ${name}: expected ${expected}, oasdiff says ${got.verdict}\n`);
    if (!ok) process.stdout.write(`${got.output}\n`);
  }
  process.stdout.write(`api-compat-selftest (--fail-on ${level}): ${names.length - failures}/${names.length} pairs as expected\n`);
  if (gaps.length > 0) {
    process.stdout.write(`api-compat-selftest: oasdiff does not report these breaking changes of VER-2: ${gaps.join(', ')}\n`);
  }
  process.stdout.write('api-compat-selftest: webhooks are compared as requests (removed events and field removals are reported, additions of required event fields are false positives); see H-API5-1\n');
  process.exit(failures === 0 ? 0 : 1);
}
