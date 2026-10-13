// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Removes the experimental operations and events from a bundled contract, so oasdiff only
 * compares what is under the compatibility guarantee (VER-1, VER-4: stable and beta).
 *
 *   node scripts/stability-filter.mjs --input <bundle> --output <filtered.yaml>
 *
 * The experimental operations are reported separately in the API changelog.
 */

import { METHODS, must, readDocument, writeYaml } from './doc-io.mjs';
import { isMain, parseArgs } from './lib.mjs';

/** Returns a copy of `doc` without experimental operations; empty path items are dropped. */
export function withoutExperimental(doc) {
  const out = structuredClone(doc);
  for (const map of [out.paths, out.webhooks]) {
    for (const [key, item] of Object.entries(map ?? {})) {
      for (const method of METHODS) {
        if (item?.[method]?.['x-orcon-stability'] === 'experimental') delete item[method];
      }
      if (!METHODS.some((m) => item?.[m])) delete map[key];
    }
  }
  return out;
}

if (isMain(import.meta.url)) {
  const { flags } = parseArgs(process.argv.slice(2));
  must(flags.input && flags.output, 'usage: stability-filter.mjs --input <bundle> --output <filtered.yaml>');
  writeYaml(flags.output, withoutExperimental(readDocument(flags.input)));
}
