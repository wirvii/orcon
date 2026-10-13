// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { join } from 'node:path';

import { packageDir, redocly } from '../../scripts/lib.mjs';

export const testConfig = join(packageDir, 'testdata', 'redocly.yaml');

/**
 * Lints a document with the self-test configuration and returns the problems:
 * [{ ruleId, severity, message, file }].
 */
export function lint(file, config = testConfig) {
  const result = redocly(['lint', file, '--config', config, '--format', 'json', '--max-problems', '1000']);
  const json = result.stdout.slice(result.stdout.indexOf('{'));
  let parsed;
  try {
    parsed = JSON.parse(json);
  } catch {
    throw new Error(`redocly lint did not print JSON for ${file}:\n${result.stdout}${result.stderr}`);
  }
  return parsed.problems.map((p) => ({
    ruleId: p.ruleId,
    severity: p.severity,
    message: p.message,
    file: p.location?.[0]?.source?.ref,
  }));
}

/** The distinct orcon rule ids among problems. */
export const orconRules = (problems) => [...new Set(problems.filter((p) => p.ruleId.startsWith('orcon/')).map((p) => p.ruleId))];
