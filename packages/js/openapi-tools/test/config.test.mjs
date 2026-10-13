// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { test } from 'node:test';

import { parse } from 'yaml';

import { RULE_NAMES } from '../plugin/orcon.js';
import { packageDir } from '../scripts/lib.mjs';

const config = parse(readFileSync(join(packageDir, 'redocly.yaml'), 'utf8'));
const severityOf = (rule) => (typeof rule === 'string' ? rule : rule.severity);

test('redocly.yaml registers the plugin and enables all 31 orcon rules as errors', () => {
  assert.deepEqual(config.plugins, ['./plugin/orcon.js']);
  for (const name of RULE_NAMES) {
    const rule = config.rules[`orcon/${name}`];
    assert.ok(rule, `orcon/${name} is not enabled`);
    assert.equal(severityOf(rule), 'error', `orcon/${name} is not an error`);
  }
  assert.equal(Object.keys(config.rules).filter((k) => k.startsWith('orcon/')).length, 31);
});

test('no rule is turned off or downgraded', () => {
  for (const [name, rule] of Object.entries(config.rules)) {
    assert.equal(severityOf(rule), 'error', `${name} is not an error`);
  }
});

test('the built-in rules of the toolchain are enabled', () => {
  const required = [
    'struct',
    'operation-operationId-unique',
    'operation-operationId-url-safe',
    'path-parameters-defined',
    'no-ambiguous-paths',
    'no-invalid-media-type-examples',
    'no-invalid-schema-examples',
    'no-unused-components',
    'security-defined',
    'no-server-trailing-slash',
    'operation-4xx-problem-details-rfc7807',
  ];
  for (const name of required) assert.equal(config.rules[name], 'error', name);
});
