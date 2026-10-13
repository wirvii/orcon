// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Bundles the split contract into single documents: YAML for the Go generator and JSON for
 * the Go tests (spec section 4.1).
 *
 *   node scripts/bundle.mjs [--input <openapi.yaml>] [--yaml <out.yaml>] [--json <out.json>]
 *
 * The input defaults to packages/openapi/v1/openapi.yaml. At least one output is needed.
 * Unused components are kept; the output is the exact bundle Redocly produces, so a drift
 * check is a plain diff.
 */

import { mkdirSync } from 'node:fs';
import { dirname, resolve } from 'node:path';

import { defaultContract, fail, isMain, parseArgs, redocly } from './lib.mjs';

/** Bundles `input` into `output` ("yaml" or "json" by extension). Returns nothing, throws on failure. */
export function bundle(input, output) {
  const ext = output.endsWith('.json') ? 'json' : 'yaml';
  mkdirSync(dirname(resolve(output)), { recursive: true });
  const result = redocly(['bundle', resolve(input), '--output', resolve(output), '--ext', ext, '--lint-config', 'off']);
  if (result.status !== 0) {
    throw new Error(`redocly bundle failed for ${input}:\n${result.stdout}${result.stderr}`);
  }
}

if (isMain(import.meta.url)) {
  const { flags } = parseArgs(process.argv.slice(2));
  const input = flags.input ?? defaultContract();
  if (!flags.yaml && !flags.json) fail('usage: bundle.mjs [--input <openapi.yaml>] [--yaml <out.yaml>] [--json <out.json>]');
  try {
    if (flags.yaml) bundle(input, flags.yaml);
    if (flags.json) bundle(input, flags.json);
  } catch (error) {
    fail(error.message);
  }
}
