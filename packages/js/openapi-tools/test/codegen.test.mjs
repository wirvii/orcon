// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { test } from 'node:test';

import { bundle } from '../scripts/bundle.mjs';
import { packageDir, repoRoot } from '../scripts/lib.mjs';

const tools = join(repoRoot(), 'tools');
const goAvailable = spawnSync('go', ['version'], { encoding: 'utf8' }).status === 0 && existsSync(join(tools, 'go.mod'));

test('the bundled good document is accepted by the pinned Go generator', { skip: goAvailable ? false : 'go is not available' }, (t) => {
  const dir = mkdtempSync(join(tmpdir(), 'orcon-codegen-'));
  try {
    const bundled = join(dir, 'openapi.bundle.yaml');
    bundle(join(packageDir, 'testdata', 'good', 'openapi.yaml'), bundled);
    const output = join(dir, 'server.gen.go');
    writeFileSync(
      join(dir, 'oapi-codegen.yaml'),
      `package: apigen\noutput: ${output}\ngenerate:\n  std-http-server: true\n  strict-server: true\n  models: true\n  embedded-spec: true\noutput-options:\n  nullable-type: true\n`,
    );
    const result = spawnSync('go', ['tool', 'oapi-codegen', '-config', join(dir, 'oapi-codegen.yaml'), bundled], {
      cwd: tools,
      encoding: 'utf8',
      // Offline and from the module cache: a cold cache skips the test instead of using the network.
      env: { ...process.env, GOWORK: 'off', GOFLAGS: '-mod=mod', GOPROXY: 'off' },
    });
    if (result.status !== 0 && /module lookup disabled|cannot find module|no required module|GOPROXY=off/.test(result.stderr)) {
      t.skip('the Go module cache is cold (oapi-codegen is not downloaded); run `go mod download` in tools/ first');
      return;
    }
    assert.equal(result.status, 0, `${result.stdout}\n${result.stderr}`);
    const code = readFileSync(output, 'utf8');
    assert.match(code, /type StrictServerInterface interface/);
    assert.match(code, /type FlowDraft struct/);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});
