// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it, vi } from 'vitest';

import { createQueryClient, onUnauthenticated } from './query-client';

/** A failure shaped like an API problem: an Error that carries the status, as the transport makes it. */
const failure = (status: number, title: string) => Object.assign(new Error(title), { status, title });

describe('onUnauthenticated', () => {
  it('reports a 401 of a query to the handler', async () => {
    const queryClient = createQueryClient();
    const handler = vi.fn();
    onUnauthenticated(queryClient, handler);
    await expect(queryClient.query({ queryKey: ['session', 'later'], queryFn: () => Promise.reject(failure(401, 'unauthenticated')), retry: false })).rejects.toMatchObject({
      status: 401,
    });
    expect(handler).toHaveBeenCalledTimes(1);
  });

  it('ignores any other failure', async () => {
    const queryClient = createQueryClient();
    const handler = vi.fn();
    onUnauthenticated(queryClient, handler);
    await expect(queryClient.query({ queryKey: ['orgs'], queryFn: () => Promise.reject(failure(500, 'internal')), retry: false })).rejects.toMatchObject({
      status: 500,
    });
    expect(handler).not.toHaveBeenCalled();
  });

  it('reports a 401 of an ordinary mutation', async () => {
    const queryClient = createQueryClient();
    const handler = vi.fn();
    onUnauthenticated(queryClient, handler);
    const mutation = queryClient.getMutationCache().build(queryClient, {
      mutationKey: ['projects', 'create'],
      mutationFn: () => Promise.reject(failure(401, 'unauthenticated')),
    });
    await expect(mutation.execute(undefined)).rejects.toMatchObject({ status: 401 });
    expect(handler).toHaveBeenCalledTimes(1);
  });

  it('does not report a 401 of a session mutation: a wrong password is an answer', async () => {
    const queryClient = createQueryClient();
    const handler = vi.fn();
    onUnauthenticated(queryClient, handler);
    const mutation = queryClient.getMutationCache().build(queryClient, {
      mutationKey: ['session', 'create'],
      mutationFn: () => Promise.reject(failure(401, 'unauthenticated')),
    });
    await expect(mutation.execute(undefined)).rejects.toMatchObject({ status: 401 });
    expect(handler).not.toHaveBeenCalled();
  });
});
