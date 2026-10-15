// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from 'vitest';

import { greetingFor } from './greeting';

describe('greetingFor', () => {
  it.each([
    [0, 'morning'],
    [11, 'morning'],
    [12, 'afternoon'],
    [17, 'afternoon'],
    [18, 'evening'],
    [23, 'evening'],
  ] as const)('greets hour %i with %s', (hour, expected) => {
    expect(greetingFor(hour)).toBe(expected);
  });
});
