// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { getModel } from './model.js';

/**
 * Wraps a check into a Redocly rule. The check receives the shared model, a
 * `report(ruleId, message, location)` function that prefixes the rulebook id and
 * de-duplicates findings reached through several operations, and the rule options from
 * `redocly.yaml`.
 */
export function defineRule(check) {
  return (options) => ({
    Root(root, ctx) {
      const model = getModel(root, ctx);
      if (Array.isArray(options?.protocolPaths)) model.setProtocolPaths(options.protocolPaths);
      const seen = new Set();
      const report = (id, message, location) => {
        const text = id ? `[${id}] ${message}` : message;
        const key = `${location.source.absoluteRef}${location.pointer}\u0000${text}`;
        if (seen.has(key)) return;
        seen.add(key);
        ctx.report({ message: text, location });
      };
      check(model, report, options ?? {});
    },
  });
}

export const KEBAB = /^[a-z][a-z0-9]*(-[a-z0-9]+)*$/;
export const CAMEL = /^[a-z][a-zA-Z0-9]*$/;
export const SNAKE = /^[a-z][a-z0-9]*(_[a-z0-9]+)*$/;
export const UPPER_SNAKE = /^[A-Z][A-Z0-9]*(_[A-Z0-9]+)*$/;

export const STABILITIES = ['stable', 'beta', 'experimental'];

/** Header names that are not Orcon headers (RESP-4 and standard HTTP). Compared case-insensitively. */
export const STANDARD_HEADERS = [
  'accept',
  'accept-language',
  'authorization',
  'cache-control',
  'content-disposition',
  'content-language',
  'content-length',
  'content-type',
  'date',
  'etag',
  'if-match',
  'if-none-match',
  'last-event-id',
  'link',
  'location',
  'retry-after',
  'idempotency-key',
  'idempotent-replayed',
  'deprecation',
  'sunset',
  'ratelimit',
  'ratelimit-policy',
  'vary',
  'allow',
  'webhook-id',
  'webhook-timestamp',
  'webhook-signature',
];

export function isError(status) {
  return status === 'default' || /^[45]/.test(status);
}

export function isSuccess(status) {
  return /^2/.test(status);
}

/** Last path literal of a path ("/flows/{flowId}/publish" -> "publish"), or undefined if it ends in a parameter. */
export function lastLiteral(path) {
  const parts = path.split('/').filter(Boolean);
  const last = parts[parts.length - 1];
  if (!last || last.startsWith('{')) return undefined;
  return last;
}
