// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Third-party notices of the reference site (SPEC-012 Amendment 2, section 2-9).
 *
 * `scalar.standalone.js` bundles @scalar/api-reference and the packages of its production
 * closure that its source map lists. This module finds those packages in the installed tree,
 * checks every license against the allowed list (Apache-2.0, MIT, BSD-2-Clause, BSD-3-Clause,
 * ISC, or an exact `package@version` line in tools/licenses/npm-exceptions.txt) and renders
 * the notices that ship next to the script as third-party-notices.txt.
 */

import { existsSync, readFileSync, readdirSync, realpathSync } from 'node:fs';
import { dirname, join, parse as parsePath } from 'node:path';

import { packageDir, repoRoot } from './lib.mjs';

export const ALLOWED_LICENSES = Object.freeze(['Apache-2.0', 'MIT', 'BSD-2-Clause', 'BSD-3-Clause', 'ISC']);
const LICENSE_FILE = /^(licen[sc]e|copying|notice)(\.(md|txt))?$/i;

/** Finds `node_modules/<name>` walking up from `from` (the pnpm layout keeps dependencies as siblings). */
export function findPackageDir(name, from) {
  let dir = from;
  for (;;) {
    const candidate = join(dir, 'node_modules', name);
    if (existsSync(join(candidate, 'package.json'))) return realpathSync(candidate);
    const parent = dirname(dir);
    if (parent === dir || dir === parsePath(dir).root) return undefined;
    dir = parent;
  }
}

/** The production closure of the Scalar package: Map("name@version" -> { name, version, dir, pkg }). */
export function scalarClosure() {
  const root = findPackageDir('@scalar/api-reference', packageDir);
  if (!root) throw new Error('@scalar/api-reference is not installed; run pnpm install');
  const closure = new Map();
  const queue = [root];
  while (queue.length > 0) {
    const dir = queue.pop();
    const pkg = JSON.parse(readFileSync(join(dir, 'package.json'), 'utf8'));
    const id = `${pkg.name}@${pkg.version}`;
    if (closure.has(id)) continue;
    closure.set(id, { name: pkg.name, version: pkg.version, dir, pkg });
    for (const dep of Object.keys({ ...pkg.dependencies, ...pkg.optionalDependencies })) {
      const found = findPackageDir(dep, dir);
      if (found) queue.push(found);
      else if (pkg.dependencies?.[dep] !== undefined) throw new Error(`dependency ${dep} of ${id} is not installed`);
    }
  }
  return closure;
}

/** Names of the node_modules packages that appear in the source map of the script. */
export function bundledNames(sourceMapFile) {
  const map = JSON.parse(readFileSync(sourceMapFile, 'utf8'));
  const names = new Set();
  for (const source of map.sources) {
    const at = source.lastIndexOf('node_modules/');
    if (at < 0) continue;
    const parts = source.slice(at + 'node_modules/'.length).split('/');
    names.add(parts[0].startsWith('@') ? `${parts[0]}/${parts[1]}` : parts[0]);
  }
  return names;
}

/** The packages whose code is inside the script: Scalar itself and the closure packages in its source map. */
export function bundledPackages(scalarScript) {
  const closure = scalarClosure();
  const names = bundledNames(`${scalarScript}.map`);
  const out = [];
  for (const entry of closure.values()) {
    if (entry.name === '@scalar/api-reference' || entry.name.startsWith('@scalar/') || names.has(entry.name)) out.push(entry);
  }
  const known = new Set(out.map((e) => e.name));
  for (const name of names) {
    if (!known.has(name)) throw new Error(`the source map lists ${name}, which is not in the closure of @scalar/api-reference`);
  }
  return out.sort((a, b) => `${a.name}@${a.version}`.localeCompare(`${b.name}@${b.version}`));
}

/** The license expression of a package.json as a string. */
export function licenseOf(pkg) {
  if (typeof pkg.license === 'string') return pkg.license;
  if (typeof pkg.license?.type === 'string') return pkg.license.type;
  if (Array.isArray(pkg.licenses)) return pkg.licenses.map((l) => l.type ?? l).join(' OR ');
  return 'UNKNOWN';
}

/** True when an SPDX expression (ids, parentheses, OR, AND) is satisfied by the allowed ids. */
export function expressionAllowed(expression, allowed = ALLOWED_LICENSES) {
  const tokens = expression.replace(/\(/g, ' ( ').replace(/\)/g, ' ) ').trim().split(/\s+/);
  let i = 0;
  const parseOr = () => {
    let value = parseAnd();
    while (tokens[i] === 'OR') {
      i += 1;
      const rhs = parseAnd();
      value = value || rhs;
    }
    return value;
  };
  const parseAnd = () => {
    let value = parsePrimary();
    while (tokens[i] === 'AND') {
      i += 1;
      const rhs = parsePrimary();
      value = value && rhs;
    }
    return value;
  };
  const parsePrimary = () => {
    if (tokens[i] === '(') {
      i += 1;
      const value = parseOr();
      i += 1; // ')'
      return value;
    }
    const id = tokens[i];
    i += 1;
    return allowed.includes(id);
  };
  return parseOr();
}

/** `package@version` lines of tools/licenses/npm-exceptions.txt (first field of each data line). */
export function loadExceptions(file = join(repoRoot(), 'tools', 'licenses', 'npm-exceptions.txt')) {
  if (!existsSync(file)) return new Set();
  return new Set(
    readFileSync(file, 'utf8')
      .split('\n')
      .filter((l) => l.trim() !== '' && !l.startsWith('#') && /approved-by:/.test(l))
      .map((l) => l.trim().split(/\s{2,}/)[0]),
  );
}

/** Problems: [message] for each package whose license is not allowed. */
export function licenseProblems(packages, exceptions = loadExceptions()) {
  const problems = [];
  for (const { name, version, pkg } of packages) {
    const license = licenseOf(pkg);
    if (!expressionAllowed(license) && !exceptions.has(`${name}@${version}`)) {
      problems.push(`third-party notices: ${name}@${version} has the license ${license}, which is not allowed (add an approved line to tools/licenses/npm-exceptions.txt or replace the package)`);
    }
  }
  return problems;
}

function licenseText(entry, scalarText) {
  if (entry.name === '@scalar/api-reference') return scalarText.trim();
  const file = readdirSync(entry.dir).find((f) => LICENSE_FILE.test(f));
  if (file) return readFileSync(join(entry.dir, file), 'utf8').trim();
  const author = typeof entry.pkg.author === 'string' ? entry.pkg.author : entry.pkg.author?.name;
  return `No license file ships with this package. License field: ${licenseOf(entry.pkg)}.${author ? ` Author: ${author}.` : ''}`;
}

/** The text of third-party-notices.txt. `scalarText` is the license text of @scalar/api-reference. */
export function renderNotices(packages, scalarText) {
  const lines = [
    'Third-party notices of the Orcon API reference site',
    '',
    'scalar.standalone.js bundles Scalar API Reference and the packages below. Each one is',
    'distributed under the license shown with it; every license is on the Orcon allowed list.',
    '',
  ];
  for (const entry of packages) {
    lines.push('--------------------------------------------------------------------------------', `${entry.name}@${entry.version} (${licenseOf(entry.pkg)})`, '', licenseText(entry, scalarText), '');
  }
  return lines.join('\n');
}
