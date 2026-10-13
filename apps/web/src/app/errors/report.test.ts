// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { afterEach, describe, expect, it, vi } from 'vitest';
import { z } from 'zod';

import { ContractError } from './contract-error';
import { reportContractError } from './report';

afterEach(() => {
  vi.restoreAllMocks();
});

describe('reportContractError', () => {
  const schema = z.object({ name: z.string(), nested: z.object({ count: z.number() }) });
  const failure = (input: unknown) => new ContractError('response of GET /flows', schema.safeParse(input).error?.issues ?? []);

  it('logs where the value came from and which paths broke which rule', () => {
    const log = vi.spyOn(console, 'error').mockImplementation(() => undefined);
    reportContractError(failure({ name: 1, nested: { count: 'x' } }));
    expect(log).toHaveBeenCalledWith('Contract error: response of GET /flows', ['name: invalid_type', 'nested.count: invalid_type']);
  });

  it('adds the request id when there is one', () => {
    const log = vi.spyOn(console, 'error').mockImplementation(() => undefined);
    reportContractError(failure(null), 'req_9');
    expect(log).toHaveBeenCalledWith('Contract error: response of GET /flows (request req_9)', ['(root): invalid_type']);
  });

  it('never logs the value itself', () => {
    const log = vi.spyOn(console, 'error').mockImplementation(() => undefined);
    reportContractError(failure({ name: 'api-key-123', nested: { count: 'api-key-123' } }));
    expect(JSON.stringify(log.mock.calls)).not.toContain('api-key-123');
  });
});
