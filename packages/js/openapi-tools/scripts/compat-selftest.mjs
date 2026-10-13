// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * The compatibility check and its self-test (R-COMPAT-01, ADR-0054 section 3, SPEC-012
 * Amendment 2 section 2-5).
 *
 * The check, as `make api-compat` runs it on two bundles:
 *   1. remove the experimental operations (scripts/stability-filter.mjs);
 *   2. turn the events into responses (scripts/events-as-responses.mjs);
 *   3. oasdiff breaking --fail-on ERR --severity-levels oasdiff-levels.txt;
 *   4. scripts/compat-extra.mjs for the changes that live in x-orcon-* extensions.
 * It is breaking when step 3 or step 4 reports something. --fail-on WARN is never used.
 *
 *   node scripts/compat-selftest.mjs [--oasdiff "<command>"] [--oasdiff-cwd <dir>] [--cases <dir>]
 *   node scripts/compat-selftest.mjs --check --base <bundle> --head <bundle> [--oasdiff ...]
 *
 * The oasdiff command defaults to $ORCON_OASDIFF, then to `oasdiff` on PATH; the make target
 * passes `go tool oasdiff` from the tools module. The self-test compares the verdict and the
 * detector of every pair in testdata/compat with the expectation recorded next to it.
 */

import { spawnSync } from 'node:child_process';
import { mkdtempSync, readFileSync, readdirSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';

import { compatExtra } from './compat-extra.mjs';
import { readDocument, writeYaml } from './doc-io.mjs';
import { eventsAsResponses } from './events-as-responses.mjs';
import { isMain, packageDir, parseArgs } from './lib.mjs';
import { withoutExperimental } from './stability-filter.mjs';

export const LEVELS_FILE = join(packageDir, 'oasdiff-levels.txt');

/** Splits a command string into [program, ...args] on whitespace. */
export const splitCommand = (command) => command.trim().split(/\s+/);

/** The `key: value` lines of an expect.yaml file as an object. */
export function expectation(file) {
  const out = {};
  for (const line of readFileSync(file, 'utf8').split('\n')) {
    const m = /^([a-z]+):\s*(\S+)\s*$/.exec(line);
    if (m) out[m[1]] = m[2];
  }
  return out;
}

/** Runs `oasdiff breaking --fail-on ERR --severity-levels <file> base head`; returns { status, output }. */
export function oasdiffBreaking(command, base, head, cwd, levelsFile = LEVELS_FILE) {
  const [program, ...prefix] = splitCommand(command);
  const result = spawnSync(program, [...prefix, 'breaking', '--fail-on', 'ERR', '--severity-levels', levelsFile, base, head], { encoding: 'utf8', cwd, env: { ...process.env, GOWORK: 'off' } });
  if (result.error) throw new Error(`cannot run ${command}: ${result.error.message}`);
  return { status: result.status ?? 1, output: `${result.stdout}${result.stderr}` };
}

/**
 * Verdict of one pair: { verdict: 'fail' | 'pass', detectors: ['oasdiff'?, 'extra'?], output }.
 * `baseDoc` and `headDoc` are bundled documents.
 */
export function compatVerdict(command, baseDoc, headDoc, cwd) {
  const dir = mkdtempSync(join(tmpdir(), 'orcon-compat-'));
  try {
    const base = join(dir, 'base.yaml');
    const head = join(dir, 'head.yaml');
    writeYaml(base, eventsAsResponses(withoutExperimental(baseDoc)));
    writeYaml(head, eventsAsResponses(withoutExperimental(headDoc)));
    const diff = oasdiffBreaking(command, base, head, cwd);
    if (diff.status > 1) throw new Error(`oasdiff failed:\n${diff.output}`);
    const extra = compatExtra(baseDoc, headDoc);
    const detectors = [];
    if (diff.status === 1) detectors.push('oasdiff');
    if (extra.length > 0) detectors.push('extra');
    const output = `${diff.status === 1 ? diff.output : ''}${extra.map((f) => `compat-extra: ${f.where}: ${f.message}\n`).join('')}`;
    return { verdict: detectors.length > 0 ? 'fail' : 'pass', detectors, output };
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
}

if (isMain(import.meta.url)) {
  const { flags } = parseArgs(process.argv.slice(2), ['check']);
  const command = flags.oasdiff ?? process.env.ORCON_OASDIFF ?? 'oasdiff';
  const cwd = flags['oasdiff-cwd'] ? resolve(flags['oasdiff-cwd']) : undefined;
  if (flags.check) {
    const got = compatVerdict(command, readDocument(flags.base), readDocument(flags.head), cwd);
    process.stdout.write(got.output);
    process.stdout.write(got.verdict === 'fail' ? 'api-compat: breaking changes on stable or beta\n' : 'api-compat: no breaking change\n');
    process.exit(got.verdict === 'fail' ? 1 : 0);
  }
  const root = resolve(flags.cases ?? join(packageDir, 'testdata', 'compat'));
  let failures = 0;
  const names = readdirSync(root, { withFileTypes: true }).filter((e) => e.isDirectory()).map((e) => e.name).sort();
  for (const name of names) {
    const expected = expectation(join(root, name, 'expect.yaml'));
    const got = compatVerdict(command, readDocument(join(root, name, 'base.yaml')), readDocument(join(root, name, 'head.yaml')), cwd);
    const ok = got.verdict === expected.expect && (expected.expect === 'pass' || got.detectors.includes(expected.detector));
    if (!ok) failures += 1;
    process.stdout.write(`${ok ? 'ok  ' : 'FAIL'} ${name}: expected ${expected.expect}${expected.detector ? ` by ${expected.detector}` : ''}, got ${got.verdict}${got.detectors.length ? ` by ${got.detectors.join('+')}` : ''}\n`);
    if (!ok) process.stdout.write(`${got.output}\n`);
  }
  process.stdout.write(`api-compat-selftest: ${names.length - failures}/${names.length} pairs as expected\n`);
  process.exit(failures === 0 ? 0 : 1);
}
