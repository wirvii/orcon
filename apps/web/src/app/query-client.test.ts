// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from 'vitest';

import { clearCacheForOrganizationSwitch, createQueryClient, GC_TIME_MS, isNetworkFailure, MAX_RETRIES, shouldRetry, STALE_TIME_MS } from './query-client';

describe('createQueryClient', () => {
  it('has the defaults of the rulebook', () => {
    const queries = createQueryClient().getDefaultOptions().queries;
    expect(queries?.staleTime).toBe(30_000);
    expect(queries?.gcTime).toBe(300_000);
    expect(queries?.refetchOnWindowFocus).toBe(true);
    expect(STALE_TIME_MS).toBe(30_000);
    expect(GC_TIME_MS).toBe(300_000);
    expect(MAX_RETRIES).toBe(2);
  });
});

describe('retrying', () => {
  it('treats a request that never reached the server as a network failure', () => {
    expect(isNetworkFailure(new TypeError('Failed to fetch'))).toBe(true);
    expect(isNetworkFailure(new Error('Server said no'))).toBe(false);
    expect(isNetworkFailure({ status: 500 })).toBe(false);
  });

  it('retries a network failure at most twice', () => {
    const failure = new TypeError('Failed to fetch');
    expect(shouldRetry(0, failure)).toBe(true);
    expect(shouldRetry(1, failure)).toBe(true);
    expect(shouldRetry(2, failure)).toBe(false);
  });

  it('never retries an answer from the server', () => {
    expect(shouldRetry(0, { status: 500 })).toBe(false);
    expect(shouldRetry(0, new Error('Server said no'))).toBe(false);
  });
});

describe('clearCacheForOrganizationSwitch', () => {
  it('drops every cached query', () => {
    const client = createQueryClient();
    client.setQueryData(['org-1', 'flows', 'list'], [1]);
    client.setQueryData(['org-1', 'flows', 'detail', 'a'], { id: 'a' });
    clearCacheForOrganizationSwitch(client);
    expect(client.getQueryCache().getAll()).toHaveLength(0);
  });
});
