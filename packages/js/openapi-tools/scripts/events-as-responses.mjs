// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Turns every event of the `webhooks` section into a synthetic, unpublished operation so
 * oasdiff compares event payloads in the right direction (SPEC-012 Amendment 2, section 2-5).
 *
 *   node scripts/events-as-responses.mjs --input <bundle> --output <converted.yaml>
 *
 * oasdiff compares a webhook like an operation whose REQUEST body is the payload: adding a
 * required field to an event (harmless for receivers) then looks breaking and removing one
 * looks harmless. Orcon sends the payload, so the payload is a RESPONSE: each event becomes
 * `GET /__events__/<event type>` whose `200` response is the payload. Removing an event type
 * or a payload field is then incompatible and adding a field is not. The converted document
 * is only an input for oasdiff; it is never published.
 */

import { METHODS, must, readDocument, writeYaml } from './doc-io.mjs';
import { isMain, parseArgs } from './lib.mjs';

export const EVENT_PATH_PREFIX = '/__events__/';

/** Returns a copy of `doc` with the webhooks converted into synthetic GET operations. */
export function eventsAsResponses(doc) {
  const out = structuredClone(doc);
  const webhooks = out.webhooks ?? {};
  delete out.webhooks;
  out.paths = out.paths ?? {};
  for (const [type, item] of Object.entries(webhooks)) {
    const op = METHODS.map((m) => item?.[m]).find(Boolean);
    if (!op) continue;
    const content = op.requestBody?.content ?? {};
    out.paths[`${EVENT_PATH_PREFIX}${type}`] = {
      get: {
        operationId: `event.${type}`,
        summary: `Event ${type}`,
        ...(op['x-orcon-stability'] ? { 'x-orcon-stability': op['x-orcon-stability'] } : {}),
        responses: { 200: { description: `The payload of the ${type} event.`, content } },
      },
    };
  }
  return out;
}

if (isMain(import.meta.url)) {
  const { flags } = parseArgs(process.argv.slice(2));
  must(flags.input && flags.output, 'usage: events-as-responses.mjs --input <bundle> --output <converted.yaml>');
  writeYaml(flags.output, eventsAsResponses(readDocument(flags.input)));
}
