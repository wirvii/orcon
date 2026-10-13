// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from 'vitest';
import { z } from 'zod';

import { ContractError } from '../errors/contract-error';

import { parseWith, tryParse } from './parse';

const flow = z.object({ id: z.string(), runs: z.number().int().min(0) });

describe('parseWith', () => {
  it('returns the parsed value', () => {
    expect(parseWith(flow, { id: 'a', runs: 3 })).toEqual({ id: 'a', runs: 3 });
  });

  it('applies schema defaults and drops unknown members', () => {
    const withDefault = z.object({ tab: z.string().default('overview') });
    expect(parseWith(withDefault, { extra: true })).toEqual({ tab: 'overview' });
  });

  it('throws a contract error that names the source and the failing paths, not the value', () => {
    try {
      parseWith(flow, { id: 'secret-token', runs: -1 }, 'response of GET /flows');
      expect.unreachable('parseWith must throw');
    } catch (error) {
      expect(error).toBeInstanceOf(ContractError);
      const contract = error as ContractError;
      expect(contract.name).toBe('ContractError');
      expect(contract.source).toBe('response of GET /flows');
      expect(contract.message).toBe('response of GET /flows does not match its schema');
      expect(contract.issues.map((issue) => issue.path.join('.'))).toEqual(['runs']);
      expect(JSON.stringify(contract.message)).not.toContain('secret-token');
    }
  });

  it('uses a generic source when none is given', () => {
    expect(() => parseWith(flow, null)).toThrow('value does not match its schema');
  });
});

describe('tryParse', () => {
  it('returns the value on success', () => {
    expect(tryParse(flow, { id: 'a', runs: 1 })).toEqual({ ok: true, value: { id: 'a', runs: 1 } });
  });

  it('returns the contract error on failure instead of throwing', () => {
    const result = tryParse(flow, { id: 1 }, 'saved value');
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.error).toBeInstanceOf(ContractError);
      expect(result.error.source).toBe('saved value');
      expect(result.error.issues.length).toBeGreaterThan(0);
    }
  });
});
