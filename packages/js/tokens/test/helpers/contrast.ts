// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/** Minimum WCAG 2.x contrast ratio per pair kind. Never lowered to make a test pass. */
export const MINIMUM = { text: 4.5, ui: 3 } as const;

export type PairKind = 'text' | 'ui' | 'decorative' | 'disabled-exempt';

export interface Pair {
  fg: string;
  bg: string;
  kind: PairKind;
  min?: number;
  reason?: string;
}

function channel(value: number): number {
  const c = value / 255;
  return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
}

/** WCAG 2.x relative luminance of a #RRGGBB color. */
export function luminance(hex: string): number {
  const match = /^#([0-9a-f]{6})$/i.exec(hex);
  if (!match) throw new Error(`Unsupported color for contrast: ${hex}`);
  const n = Number.parseInt(match[1] as string, 16);
  return 0.2126 * channel((n >> 16) & 255) + 0.7152 * channel((n >> 8) & 255) + 0.0722 * channel(n & 255);
}

export function ratio(a: string, b: string): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x) as [number, number];
  return (hi + 0.05) / (lo + 0.05);
}

/** Returns one message per problem found in the pairs for one theme; empty means they all hold. */
export function evaluatePairs(pairs: Pair[], values: Record<string, string>): string[] {
  const problems: string[] = [];
  for (const pair of pairs) {
    const label = `${pair.fg} on ${pair.bg}`;
    const fg = values[pair.fg];
    const bg = values[pair.bg];
    if (fg === undefined || bg === undefined) {
      problems.push(`${label}: unknown token`);
      continue;
    }
    if (pair.kind === 'decorative' || pair.kind === 'disabled-exempt') {
      if (!pair.reason) problems.push(`${label}: ${pair.kind} pair needs a reason`);
      continue;
    }
    const required = MINIMUM[pair.kind as 'text' | 'ui'];
    if (required === undefined) {
      problems.push(`${label}: unknown kind "${pair.kind}"`);
      continue;
    }
    if (pair.min === undefined || pair.min < required) {
      problems.push(`${label}: minimum ${pair.min} is below the ${required} required for ${pair.kind}`);
    }
    const actual = ratio(fg, bg);
    if (actual < Math.max(pair.min ?? 0, required)) {
      problems.push(`${label}: ratio ${actual.toFixed(2)} is below ${Math.max(pair.min ?? 0, required)}`);
    }
  }
  return problems;
}
