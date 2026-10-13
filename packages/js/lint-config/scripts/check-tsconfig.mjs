// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Checks the effective TypeScript configuration of every workspace package under
// apps/ and packages/js/ against the required flags (orcon-web TS-1). It runs
// `tsc --showConfig` per package, so inheritance and local overrides are both seen.
//
// Usage: node packages/js/lint-config/scripts/check-tsconfig.mjs [--root <dir>]

import { execFileSync } from 'node:child_process';
import { existsSync, readdirSync } from 'node:fs';
import { createRequire } from 'node:module';
import { dirname, join, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const require = createRequire(import.meta.url);

/** The flags TS-1 requires, with the value each must have in the effective configuration. */
export const requiredFlags = {
  strict: true,
  noUncheckedIndexedAccess: true,
  exactOptionalPropertyTypes: true,
  noImplicitOverride: true,
  noImplicitReturns: true,
  noFallthroughCasesInSwitch: true,
  noPropertyAccessFromIndexSignature: true,
  noUnusedLocals: true,
  noUnusedParameters: true,
  useUnknownInCatchVariables: true,
  allowUnreachableCode: false,
  verbatimModuleSyntax: true,
  isolatedModules: true,
  erasableSyntaxOnly: true,
  forceConsistentCasingInFileNames: true,
};

/** Finds every directory with a tsconfig.json directly under apps/* and packages/js/*. */
export function findPackages(root) {
  const found = [];
  for (const group of ['apps', join('packages', 'js')]) {
    const dir = join(root, group);
    if (!existsSync(dir)) continue;
    for (const entry of readdirSync(dir, { withFileTypes: true })) {
      const candidate = join(dir, entry.name);
      if (entry.isDirectory() && existsSync(join(candidate, 'tsconfig.json'))) found.push(candidate);
    }
  }
  return found.sort();
}

/** Returns the problems of one package, as human-readable lines. */
export function checkPackage(packageDir, tsc) {
  let shown;
  try {
    shown = JSON.parse(
      execFileSync(process.execPath, [tsc, '--showConfig', '-p', packageDir], {
        encoding: 'utf8',
        maxBuffer: 64 * 1024 * 1024,
      }),
    );
  } catch (error) {
    return [`tsc --showConfig failed: ${error instanceof Error ? error.message : String(error)}`];
  }
  const options = shown.compilerOptions ?? {};
  const problems = [];
  for (const [flag, wanted] of Object.entries(requiredFlags)) {
    // `strict` may also be satisfied by an explicit strict set; the flag itself must be on.
    if (options[flag] !== wanted) {
      problems.push(`${flag} must be ${String(wanted)} (effective value: ${String(options[flag])})`);
    }
  }
  return problems;
}

function main() {
  const rootArg = process.argv.indexOf('--root');
  const root = resolve(rootArg > -1 ? (process.argv[rootArg + 1] ?? '.') : join(here, '..', '..', '..', '..'));
  const tsc = require.resolve('typescript/bin/tsc');
  const packages = findPackages(root);
  if (packages.length === 0) {
    console.error(`check-tsconfig: no package with a tsconfig.json under ${root}`);
    process.exit(1);
  }
  let failed = 0;
  for (const packageDir of packages) {
    const name = relative(root, packageDir);
    const problems = checkPackage(packageDir, tsc);
    if (problems.length === 0) {
      console.log(`ok    ${name}`);
      continue;
    }
    failed += 1;
    console.error(`FAIL  ${name}`);
    for (const problem of problems) console.error(`      ${problem}`);
  }
  if (failed > 0) {
    console.error(
      `\ncheck-tsconfig: ${failed} package(s) miss required flags. Extend @orcon/lint-config/tsconfig.base.json and do not turn flags off.`,
    );
    process.exit(1);
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main();
