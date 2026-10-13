// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// The license gate for shipped JavaScript (orcon-web DP-2). Two nets:
//
//   1. the production dependency closure of @orcon/web (its own dependencies and those of the
//      workspace packages it depends on), from `pnpm licenses list --prod --json`;
//   2. the packages Rollup really bundled, from apps/web/dist/.vite/bundled-modules.json (run the
//      build first). A development package imported by mistake from shipped code shows up here.
//
// A package passes when its license is Apache-2.0, MIT, BSD-2-Clause, BSD-3-Clause or ISC, or when
// tools/licenses/npm-exceptions.txt has a line for its exact name and version. GPL, AGPL, LGPL, SSPL,
// BUSL, Elastic, Commons Clause and source-available licenses fail even with a line.
//
//   node packages/js/lint-config/scripts/check-licenses.mjs [--root <repo>] [--exceptions <file>]

import { execFileSync } from 'node:child_process';
import { existsSync, readdirSync, readFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

/** Licenses shipped code may use without an exception. */
export const allowedLicenses = new Set(['Apache-2.0', 'MIT', 'BSD-2-Clause', 'BSD-3-Clause', 'ISC']);

/** True when a single license id is on the allowed list; SPDX ids are not case-sensitive. */
export function isAllowedTerm(term) {
  return [...allowedLicenses].some((id) => id.toLowerCase() === term.toLowerCase());
}

/** Licenses no exception can approve. */
const neverAllowed = [
  /(^|[^A-Za-z])A?GPL/i,
  /(^|[^A-Za-z])LGPL/i,
  /SSPL/i,
  /BUSL|Business Source/i,
  /Elastic/i,
  /Commons[- ]?Clause/i,
  /PolyForm/i,
  /\bFSL\b|Functional Source/i,
  /source[- ]available/i,
  /^SEE LICENSE/i,
  /proprietary|commercial/i,
  /UNLICENSED/i,
];

/** True for a license that no exception can approve. */
export function isNeverAllowed(license) {
  return neverAllowed.some((pattern) => pattern.test(license));
}

// ---------------------------------------------------------------------------
// SPDX expressions

/** Splits an SPDX expression into tokens: terms, AND, OR and parentheses. */
function tokenize(expression) {
  return expression.match(/\(|\)|[^\s()]+/g) ?? [];
}

/**
 * Evaluates an SPDX expression: `ok(term)` decides a single license. AND needs both sides, OR needs
 * one, AND binds tighter than OR, and an expression that cannot be parsed is not allowed.
 */
export function expressionAllowed(expression, ok) {
  const tokens = tokenize(expression);
  let at = 0;
  const peek = () => tokens[at];
  const parseOr = () => {
    let value = parseAnd();
    while (peek()?.toUpperCase() === 'OR') {
      at += 1;
      const right = parseAnd();
      value = value || right;
    }
    return value;
  };
  const parseAnd = () => {
    let value = parseAtom();
    while (peek()?.toUpperCase() === 'AND') {
      at += 1;
      const right = parseAtom();
      value = value && right;
    }
    return value;
  };
  const parseAtom = () => {
    const token = peek();
    if (token === undefined) throw new Error('unexpected end');
    at += 1;
    if (token === '(') {
      const value = parseOr();
      if (peek() !== ')') throw new Error('missing )');
      at += 1;
      return value;
    }
    return ok(token);
  };
  try {
    const value = parseOr();
    return at === tokens.length && value;
  } catch {
    return false;
  }
}

// ---------------------------------------------------------------------------
// Exceptions file

const lineFormat = /^(\S+@\S+) {2,}(\S+) {2,}(.+?) {2,}(\d{4}-\d{2}-\d{2}) {2,}approved-by:(\S+)$/;

/**
 * Reads tools/licenses/npm-exceptions.txt: `<package>@<version>  <SPDX id>  <reason>  <YYYY-MM-DD>  approved-by:<owner>`
 * with two or more spaces between fields. `#` lines and blank lines are comments.
 */
export function parseExceptions(text) {
  const entries = new Map();
  const problems = [];
  text.split('\n').forEach((raw, index) => {
    const line = raw.trim();
    if (line === '' || line.startsWith('#')) return;
    const match = lineFormat.exec(line);
    const where = `line ${index + 1}`;
    if (!match) {
      problems.push(`npm-exceptions.txt ${where} is malformed (need: <package>@<version>  <SPDX id>  <reason>  <YYYY-MM-DD>  approved-by:<owner>): ${line}`);
      return;
    }
    const [, key, license, reason, date, approvedBy] = match;
    if (Number.isNaN(Date.parse(date))) {
      problems.push(`npm-exceptions.txt ${where} has an invalid date: ${date}`);
      return;
    }
    if (entries.has(key)) {
      problems.push(`npm-exceptions.txt ${where} repeats ${key}`);
      return;
    }
    entries.set(key, { license, reason, date, approvedBy });
  });
  return { entries, problems };
}

// ---------------------------------------------------------------------------
// Evaluation

/**
 * Checks every package. `packages` are `{ name, version, license, source }`; `exceptions` is the
 * parsed exceptions map. Returns the problems found and the packages that passed by exception.
 */
export function evaluate(packages, exceptions) {
  const problems = [];
  const excepted = [];
  const used = new Set();
  const seen = new Set();
  for (const pkg of packages) {
    const key = `${pkg.name}@${pkg.version}`;
    const id = `${key} (${pkg.license || 'no license'}) [${pkg.source}]`;
    if (seen.has(`${key}|${pkg.source}`)) continue;
    seen.add(`${key}|${pkg.source}`);
    const license = pkg.license ?? '';
    if (license === '') {
      problems.push(`${id}: declares no license`);
      continue;
    }
    // An OR with an allowed alternative is fine; the other alternative is simply not used.
    if (expressionAllowed(license, isAllowedTerm)) continue;
    if (isNeverAllowed(license)) {
      problems.push(`${id}: has a license that no exception can approve`);
      continue;
    }
    const exception = exceptions.get(key);
    if (!exception) {
      problems.push(`${id}: license is not allowed and ${key} has no line in tools/licenses/npm-exceptions.txt`);
      continue;
    }
    if (exception.license !== license) {
      problems.push(`${id}: the exception line says ${exception.license}, the package says ${license}; renew the line`);
      continue;
    }
    used.add(key);
    excepted.push(key);
  }
  return { problems, excepted, used };
}

// ---------------------------------------------------------------------------
// Sources of packages

/** Workspace packages by name: `{ name: directory }`. */
function workspacePackages(root) {
  const found = new Map();
  for (const group of ['apps', join('packages', 'js')]) {
    const dir = join(root, group);
    if (!existsSync(dir)) continue;
    for (const entry of readdirSync(dir, { withFileTypes: true })) {
      const manifest = join(dir, entry.name, 'package.json');
      if (entry.isDirectory() && existsSync(manifest)) found.set(JSON.parse(readFileSync(manifest, 'utf8')).name, join(dir, entry.name));
    }
  }
  return found;
}

/** The workspace packages @orcon/web ships: itself and every workspace package in its `dependencies`, recursively. */
export function shippedWorkspacePackages(root, start = '@orcon/web') {
  const all = workspacePackages(root);
  const shipped = new Set();
  const visit = (name) => {
    if (shipped.has(name) || !all.has(name)) return;
    shipped.add(name);
    const manifest = JSON.parse(readFileSync(join(all.get(name), 'package.json'), 'utf8'));
    for (const [dependency, range] of Object.entries(manifest.dependencies ?? {})) {
      if (String(range).startsWith('workspace:')) visit(dependency);
    }
  };
  visit(start);
  return [...shipped];
}

/** `pnpm licenses list --prod --json` for one workspace package, as `{ name, version, license }` entries. */
function pnpmLicenses(root, workspaceName) {
  const output = execFileSync('pnpm', ['--filter', workspaceName, 'licenses', 'list', '--prod', '--json'], {
    cwd: root,
    encoding: 'utf8',
    maxBuffer: 64 * 1024 * 1024,
    stdio: ['ignore', 'pipe', 'ignore'],
  });
  // A package without production dependencies prints a sentence instead of JSON.
  if (!output.trimStart().startsWith('{')) return [];
  const byLicense = JSON.parse(output);
  return Object.values(byLicense).flatMap((list) =>
    list.flatMap((entry) => entry.versions.map((version) => ({ name: entry.name, version, license: entry.license, source: 'production closure' }))),
  );
}

/** The packages Rollup bundled, from the build output. */
function bundledPackages(root) {
  const file = join(root, 'apps', 'web', 'dist', '.vite', 'bundled-modules.json');
  if (!existsSync(file)) {
    throw new Error(`${file} does not exist. Build the app first (pnpm --filter @orcon/web build).`);
  }
  return JSON.parse(readFileSync(file, 'utf8')).map((entry) => ({ ...entry, source: 'bundled' }));
}

function main() {
  const here = dirname(fileURLToPath(import.meta.url));
  const rootArg = process.argv.indexOf('--root');
  const root = resolve(rootArg > -1 ? (process.argv[rootArg + 1] ?? '.') : join(here, '..', '..', '..', '..'));
  const exceptionsArg = process.argv.indexOf('--exceptions');
  const exceptionsFile = resolve(exceptionsArg > -1 ? (process.argv[exceptionsArg + 1] ?? '') : join(root, 'tools', 'licenses', 'npm-exceptions.txt'));

  const parsed = existsSync(exceptionsFile) ? parseExceptions(readFileSync(exceptionsFile, 'utf8')) : { entries: new Map(), problems: [] };
  const workspaces = shippedWorkspacePackages(root);
  const closure = workspaces.flatMap((name) => pnpmLicenses(root, name));
  const packages = [...closure, ...bundledPackages(root)];

  const { problems, excepted } = evaluate(packages, parsed.entries);
  const all = [...parsed.problems, ...problems];
  if (all.length > 0) {
    console.error(`check-licenses: ${all.length} problem(s)`);
    for (const problem of all) console.error(`  - ${problem}`);
    console.error('\nReplace the package, or ask the maintainers for an exception (a pull request that adds a line to tools/licenses/npm-exceptions.txt).');
    process.exit(1);
  }
  console.log(`check-licenses ok: ${closure.length} production packages and ${packages.length - closure.length} bundled, ${excepted.length} by exception`);
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main();
