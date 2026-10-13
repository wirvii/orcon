// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { spawnSync } from 'node:child_process';
import { existsSync } from 'node:fs';
import { join } from 'node:path';

import { repoRoot } from '../../scripts/lib.mjs';

const works = (command, cwd) => {
  const [program, ...args] = command.split(/\s+/);
  const result = spawnSync(program, [...args, '--version'], { encoding: 'utf8', cwd, env: { ...process.env, GOWORK: 'off' } });
  return result.status === 0;
};

/**
 * How to run oasdiff in this environment: { command, cwd } or undefined.
 * Order: $ORCON_OASDIFF, `oasdiff` on PATH, `go tool oasdiff` of the tools module (once the
 * make cut adds the tool directive).
 */
export function findOasdiff() {
  if (process.env.ORCON_OASDIFF && works(process.env.ORCON_OASDIFF)) return { command: process.env.ORCON_OASDIFF };
  if (works('oasdiff')) return { command: 'oasdiff' };
  const tools = join(repoRoot(), 'tools');
  if (existsSync(join(tools, 'go.mod')) && works('go tool oasdiff', tools)) return { command: 'go tool oasdiff', cwd: tools };
  return undefined;
}
