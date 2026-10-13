// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { spawnSync } from 'node:child_process';
import { existsSync, readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

export const packageDir = resolve(dirname(fileURLToPath(import.meta.url)), '..');

/** The repository root: the nearest ancestor holding pnpm-workspace.yaml. */
export function repoRoot() {
  let dir = packageDir;
  for (let i = 0; i < 8; i += 1) {
    if (existsSync(join(dir, 'pnpm-workspace.yaml'))) return dir;
    dir = dirname(dir);
  }
  return process.cwd();
}

export const defaultContract = () => join(repoRoot(), 'packages', 'openapi', 'v1', 'openapi.yaml');

const require = createRequire(import.meta.url);

/** Absolute path of the Redocly CLI entry point installed for this package. */
export function redoclyEntry() {
  const pkgPath = require.resolve('@redocly/cli/package.json');
  const pkg = JSON.parse(readFileSync(pkgPath, 'utf8'));
  const bin = typeof pkg.bin === 'string' ? pkg.bin : pkg.bin.redocly;
  return join(dirname(pkgPath), bin);
}

/** Runs the Redocly CLI and returns { status, stdout, stderr }. */
export function redocly(args, options = {}) {
  const result = spawnSync(process.execPath, [redoclyEntry(), ...args], {
    encoding: 'utf8',
    maxBuffer: 64 * 1024 * 1024,
    cwd: options.cwd ?? packageDir,
    env: { ...process.env, NO_COLOR: '1', FORCE_COLOR: '0', REDOCLY_TELEMETRY: 'off', REDOCLY_SUPPRESS_UPDATE_NOTICE: 'true' },
  });
  if (result.error) throw result.error;
  return { status: result.status ?? 1, stdout: result.stdout, stderr: result.stderr };
}

/** Minimal flag parser: --name value, --flag, and positional arguments. */
export function parseArgs(argv, booleans = []) {
  const flags = {};
  const positional = [];
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (arg.startsWith('--')) {
      const name = arg.slice(2);
      if (booleans.includes(name)) flags[name] = true;
      else flags[name] = argv[(i += 1)];
    } else {
      positional.push(arg);
    }
  }
  return { flags, positional };
}

/** True when the module is the script node was started with. */
export function isMain(metaUrl) {
  return process.argv[1] !== undefined && metaUrl === pathToFileURL(process.argv[1]).href;
}

export function fail(message) {
  process.stderr.write(`${message}\n`);
  process.exit(1);
}
