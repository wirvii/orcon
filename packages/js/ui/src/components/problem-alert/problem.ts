// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { defineMessages, type MessageDescriptor } from 'react-intl';

/** An `application/problem+json` body (RFC 9457) with the Orcon members the UI reads. */
export interface ProblemDetails {
  type?: string | undefined;
  title?: string | undefined;
  status?: number | undefined;
  detail?: string | undefined;
  instance?: string | undefined;
  /** Stable machine code. A catalog key `problem.<code>` translates it. */
  code?: string | undefined;
  requestId?: string | undefined;
  /** Per-field errors. `pointer` is a JSON pointer into the request body, such as "/name". */
  errors?: readonly { pointer: string; detail: string; code?: string }[] | undefined;
}

/** Messages for known problem codes. A code without a message falls back to the problem title. */
export const problemMessages = defineMessages({
  generic: {
    id: 'problem.generic',
    defaultMessage: 'Something went wrong.',
    description: 'Shown for a server error that has neither a translation nor a title.',
  },
  attentionAlreadyResolved: {
    id: 'problem.attention-already-resolved',
    defaultMessage: 'Someone already resolved this request.',
    description: 'Shown when a person tries to decide an attention request that another person already decided.',
  },
});

/**
 * Problem codes that have a catalog message, keyed by the `code` the server
 * sends. To translate a new code, add its message above and list it here.
 */
export const knownProblemMessages: Readonly<Record<string, MessageDescriptor>> = {
  'attention-already-resolved': problemMessages.attentionAlreadyResolved,
};

/** Deterministic control id for a field name or JSON pointer, so an error summary can link to it. */
export function fieldIdForName(name: string): string {
  return `field-${name.replace(/^\//, '').replace(/[^a-zA-Z0-9]+/g, '-').replace(/^-+|-+$/g, '') || 'root'}`;
}

/** Field errors of a problem, ready for ErrorSummary. */
export function problemFieldErrors(problem: ProblemDetails): { fieldId: string; message: string }[] {
  return (problem.errors ?? []).map((error) => ({ fieldId: fieldIdForName(error.pointer), message: error.detail }));
}

/** Field errors of a problem keyed by pointer. */
export function errorsByPointer(problem: ProblemDetails): Record<string, string> {
  return Object.fromEntries((problem.errors ?? []).map((error) => [error.pointer, error.detail]));
}
