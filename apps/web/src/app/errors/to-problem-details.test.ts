// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from 'vitest';
import { z } from 'zod';

import { ContractError } from './contract-error';
import { problemStatus, toProblemDetails } from './to-problem-details';

describe('toProblemDetails', () => {
  it('reads a problem+json body', () => {
    expect(
      toProblemDetails({
        type: 'https://orcon.dev/problems/not-found',
        title: 'Not found',
        status: 404,
        detail: 'No flow with that id.',
        instance: '/api/v1/flows/x',
        code: 'flow-not-found',
        requestId: 'req_1',
      }),
    ).toEqual({
      type: 'https://orcon.dev/problems/not-found',
      title: 'Not found',
      status: 404,
      detail: 'No flow with that id.',
      instance: '/api/v1/flows/x',
      code: 'flow-not-found',
      requestId: 'req_1',
    });
  });

  it('maps the API field error message to the UI detail member', () => {
    const problem = toProblemDetails({
      status: 422,
      errors: [
        { pointer: '/name', code: 'too-short', message: 'Use at least 3 characters.' },
        { pointer: '/settings/timeout', message: 'Must be positive.' },
      ],
    });
    expect(problem.errors).toEqual([
      { pointer: '/name', code: 'too-short', detail: 'Use at least 3 characters.' },
      { pointer: '/settings/timeout', detail: 'Must be positive.' },
    ]);
  });

  it('accepts a field error that already uses detail, and one without any text', () => {
    expect(toProblemDetails({ errors: [{ pointer: '/a', detail: 'Wrong.' }, { pointer: '/b' }] }).errors).toEqual([
      { pointer: '/a', detail: 'Wrong.' },
      { pointer: '/b', detail: '' },
    ]);
  });

  it('turns a contract error into a code-only problem the UI shows as the generic message', () => {
    const contract = new ContractError('response of GET /flows', z.string().safeParse(1).error?.issues ?? []);
    expect(toProblemDetails(contract)).toEqual({ code: 'contract-error' });
  });

  it.each([new Error('boom'), new TypeError('Failed to fetch'), null, undefined, 'text', 42, { status: 'oops' }])(
    'gives an empty problem for an error without a problem body (%s)',
    (error) => {
      expect(toProblemDetails(error)).toEqual({});
    },
  );
});

describe('problemStatus', () => {
  it('returns the HTTP status of a problem', () => {
    expect(problemStatus({ status: 403 })).toBe(403);
  });

  it('returns nothing for any other failure', () => {
    expect(problemStatus(new TypeError('Failed to fetch'))).toBeUndefined();
  });
});
