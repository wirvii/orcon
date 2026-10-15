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
  serviceUnavailable: {
    id: 'problem.service-unavailable',
    defaultMessage: 'The service is not available right now. Try again in a few minutes.',
    description: 'Shown when the server answers that an operation is not available yet or the service is down.',
  },
  internalError: {
    id: 'problem.internal-error',
    defaultMessage: 'Something went wrong on our side. Try again in a moment.',
    description: 'Shown when the server failed unexpectedly.',
  },
  rateLimited: {
    id: 'problem.rate-limited',
    defaultMessage: 'Too many requests. Wait a moment and try again.',
    description: 'Shown when the server limits how often a person can call it.',
  },
  unauthenticated: {
    id: 'problem.unauthenticated',
    defaultMessage: 'Your session ended. Sign in again.',
    description: 'Shown when the request has no valid session.',
  },
  forbidden: {
    id: 'problem.forbidden',
    defaultMessage: 'You do not have permission to do this.',
    description: 'Shown when the person lacks the permission for an action.',
  },
  notFound: {
    id: 'problem.not-found',
    defaultMessage: 'We could not find what you asked for.',
    description: 'Shown when the requested item does not exist or is not visible to the person.',
  },
  conflict: {
    id: 'problem.conflict',
    defaultMessage: 'Someone changed this at the same time. Reload and try again.',
    description: 'Shown when a change collides with another change.',
  },
  preconditionFailed: {
    id: 'problem.precondition-failed',
    defaultMessage: 'This changed since you opened it. Reload and try again.',
    description: 'Shown when the item changed since the person loaded it.',
  },
  payloadTooLarge: {
    id: 'problem.payload-too-large',
    defaultMessage: 'What you sent is too large.',
    description: 'Shown when the request body exceeds the server limit.',
  },
  csrfCheckFailed: {
    id: 'problem.csrf-check-failed',
    defaultMessage: 'The page is out of date. Reload it and try again.',
    description: 'Shown when the anti-forgery check of a change fails.',
  },
  stepUpRequired: {
    id: 'problem.step-up-required',
    defaultMessage: 'Confirm your identity again to continue.',
    description: 'Shown when an action needs a recent re-authentication.',
  },
});

/**
 * Problem codes that have a catalog message, keyed by the `code` the server
 * sends. To translate a new code, add its message above and list it here.
 */
export const knownProblemMessages: Readonly<Record<string, MessageDescriptor>> = {
  'attention-already-resolved': problemMessages.attentionAlreadyResolved,
  'service-unavailable': problemMessages.serviceUnavailable,
  'internal-error': problemMessages.internalError,
  'rate-limited': problemMessages.rateLimited,
  'unauthenticated': problemMessages.unauthenticated,
  'forbidden': problemMessages.forbidden,
  'not-found': problemMessages.notFound,
  'conflict': problemMessages.conflict,
  'precondition-failed': problemMessages.preconditionFailed,
  'payload-too-large': problemMessages.payloadTooLarge,
  'csrf-check-failed': problemMessages.csrfCheckFailed,
  'step-up-required': problemMessages.stepUpRequired,
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
