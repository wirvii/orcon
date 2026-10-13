// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import assert from 'node:assert/strict';
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { after, before, test } from 'node:test';

import { loadPermissions } from '../plugin/rules/operations.js';
import { packageDir, redocly } from '../scripts/lib.mjs';

let dir;
let counter = 0;
before(() => {
  dir = mkdtempSync(join(tmpdir(), 'orcon-permissions-'));
});
after(() => rmSync(dir, { recursive: true, force: true }));

const write = (text) => {
  const file = join(dir, `permissions-${(counter += 1)}.yaml`);
  writeFileSync(file, text);
  return file;
};

test('the catalog of the fixtures is read: names of the three scopes, roles ignored', () => {
  const { names } = loadPermissions(join(packageDir, 'testdata', 'permissions.yaml'));
  for (const name of ['org:read', 'flow:publish', 'instance:connectors', 'apikey:manage-org']) assert.ok(names.has(name), name);
  assert.ok(!names.has('flow:peek'));
});

const STRICT = [
  ['no version', 'permissions:\n  project:\n    - name: flow:read\n', 'version must be 1'],
  ['another version', 'version: 2\npermissions:\n  project:\n    - name: flow:read\n', 'version must be 1'],
  ['an unknown scope', 'version: 1\npermissions:\n  team:\n    - name: flow:read\n', 'unknown scope "team"'],
  ['an item without a name', 'version: 1\npermissions:\n  project:\n    - note: x\n', 'needs a name'],
  ['a plain string item', 'version: 1\npermissions:\n  project:\n    - flow:read\n', 'needs a name'],
  ['a name of the wrong form', 'version: 1\npermissions:\n  project:\n    - name: FlowRead\n', 'needs a name'],
  ['a scope that is not a list', 'version: 1\npermissions:\n  project: {}\n', 'must be a list'],
  ['no permissions mapping', 'version: 1\n', 'permissions must be a mapping'],
];

for (const [description, text, message] of STRICT) {
  test(`the loader fails on ${description}`, () => {
    assert.throws(() => loadPermissions(write(text)), (e) => e.message.includes(message));
  });
}

test('the loader fails when the file is missing', () => {
  assert.throws(() => loadPermissions(join(dir, 'missing.yaml')), /not found/);
});

test('a malformed catalog stops the lint with a configuration error', () => {
  const file = write('version: 3\npermissions: {}\n');
  const config = join(dir, 'redocly-bad.yaml');
  writeFileSync(config, `extends:\n  - ${join(packageDir, 'redocly.yaml')}\nrules:\n  orcon/permission:\n    severity: error\n    permissionsFile: ${file}\n`);
  const result = redocly(['lint', join(packageDir, 'testdata', 'good', 'openapi.yaml'), '--config', config]);
  assert.notEqual(result.status, 0);
  assert.match(`${result.stdout}${result.stderr}`, /version must be 1/);
});
