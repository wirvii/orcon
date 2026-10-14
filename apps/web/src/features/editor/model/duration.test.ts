// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from 'vitest';

import { formatDuration, parseDuration } from './duration';

describe('durations', () => {
  it('reads weeks, days, hours, minutes and seconds', () => {
    expect(parseDuration('PT30S')).toBe(30);
    expect(parseDuration('PT1H30M')).toBe(5400);
    expect(parseDuration('P2D')).toBe(172_800);
    expect(parseDuration('P1DT2H3M4S')).toBe(86_400 + 7_200 + 180 + 4);
    expect(parseDuration('P2W')).toBe(1_209_600);
  });

  it('refuses what the flow format refuses', () => {
    for (const text of ['', 'P', 'PT', 'P1Y', 'P1M', 'P1W2D', '30', 'pt30s', 'PT-5S', 'P1DT']) expect(parseDuration(text), text).toBeUndefined();
  });

  it('writes the shortest form and reads it back', () => {
    expect(formatDuration(1)).toBe('PT1S');
    expect(formatDuration(90)).toBe('PT1M30S');
    expect(formatDuration(3600)).toBe('PT1H');
    expect(formatDuration(86_400)).toBe('P1D');
    expect(formatDuration(90_061)).toBe('P1DT1H1M1S');
    for (const seconds of [1, 59, 60, 61, 3599, 3600, 86_399, 86_400, 200_000]) expect(parseDuration(formatDuration(seconds))).toBe(seconds);
  });

  it('never writes zero or a negative length', () => {
    expect(formatDuration(0)).toBe('PT1S');
    expect(formatDuration(-4)).toBe('PT1S');
  });
});
