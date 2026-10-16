// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from 'vitest';

import { durationOf, runStatusKey, tailId } from './format';
import { executionsSearchSchema, rangeStart, readSearchText } from './params';

describe('the address of the list of runs', () => {
  it('keeps what is valid and drops what is not', () => {
    const flowId = '0195f1a0-0000-7000-8000-0000000000e1';
    expect(executionsSearchSchema.parse({ show: 'review', range: '30d', flowId })).toEqual({ show: 'review', range: '30d', flowId });
    expect(executionsSearchSchema.parse({ show: 'everything', range: 'forever', flowId: 'x', q: 'phone' })).toEqual({});
  });

  it('counts a date range back from now', () => {
    const now = new Date('2026-10-14T12:00:00Z');
    expect(rangeStart('24h', now)).toBe('2026-10-13T12:00:00.000Z');
    expect(rangeStart('7d', now)).toBe('2026-10-07T12:00:00.000Z');
    expect(rangeStart('30d', now)).toBe('2026-09-14T12:00:00.000Z');
  });

  it('reads the search box as a whole run identifier or nothing', () => {
    expect(readSearchText('  ')).toEqual({ kind: 'empty' });
    expect(readSearchText(' 0195F1A0-0000-7000-8000-0000000000F1 ')).toEqual({ kind: 'id', id: '0195f1a0-0000-7000-8000-0000000000f1' });
    expect(readSearchText('+57 300 1234567')).toEqual({ kind: 'invalid' });
    expect(readSearchText('0195f1a0')).toEqual({ kind: 'invalid' });
  });
});

describe('how a run is named', () => {
  it('shows a run with an operation of unknown outcome as needing review, whatever the engine says', () => {
    expect(runStatusKey({ status: 'SUCCEEDED', hasUncertainOperation: true })).toBe('UNCERTAIN');
    expect(runStatusKey({ status: 'FAILED', hasUncertainOperation: true })).toBe('UNCERTAIN');
    expect(runStatusKey({ status: 'FAILED', hasUncertainOperation: false })).toBe('FAILED');
    expect(runStatusKey({ status: 'RUNNING' })).toBe('RUNNING');
  });

  it('measures a duration only when the run both started and finished', () => {
    expect(durationOf('2026-10-14T09:00:00Z', '2026-10-14T09:00:04Z')).toBe(4000);
    expect(durationOf('2026-10-14T09:00:00Z', undefined)).toBeUndefined();
    expect(durationOf('2026-10-14T09:00:04Z', '2026-10-14T09:00:00Z')).toBeUndefined();
  });

  it('tells runs that started together apart by the end of the identifier', () => {
    expect(tailId('0195f1a0-0000-7000-8000-0000000000f1')).toBe('000000f1');
  });
});
