// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * The closed vocabulary of action verbs (PATH-5, ADR 0080 section 2).
 *
 * The list is the single source of truth for the `orcon/action-verb` and
 * `orcon/operation-id` rules. `test/verbs.test.mjs` proves it is identical to the
 * PATH-5 table of `.agents/skills/orcon-api/references/paths.md`; adding a verb
 * amends ADR 0080, that table and this list in the same pull request.
 */
export const VERBS = Object.freeze([
  'activate',
  'deactivate',
  'enable',
  'disable',
  'pause',
  'resume',
  'publish',
  'rollback',
  'move',
  'cancel',
  'retry',
  'continue',
  'repeat',
  'acknowledge',
  'resolve',
  'complete',
  'signal',
  'approve',
  'reject',
  'rotate',
  'revoke',
  'test',
  'validate',
  'evaluate',
  'export',
  'sync',
  'redeliver',
  'replay',
  'upgrade',
  'authorize',
  'reindex',
]);

/** Verbs that run something and therefore use the `run` rate class (RATE-2). */
export const RUN_VERBS = Object.freeze(['retry', 'repeat', 'redeliver', 'replay']);

/** Action verbs that are read-only and may be exposed to MCP at level `read` (CHAN-5). */
export const READ_VERBS = Object.freeze(['validate', 'evaluate']);

/** Initial verbs of an operationId besides the action verbs (PATH-7). */
export const CRUD_VERBS = Object.freeze(['list', 'get', 'create', 'update', 'replace', 'delete', 'stream']);
