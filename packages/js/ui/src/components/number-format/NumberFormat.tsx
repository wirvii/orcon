// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { useIntl } from 'react-intl';

export interface NumberFormatProps {
  value: number;
  format?: 'decimal' | 'percent' | 'currency';
  /** ISO 4217 code. Required with format="currency". */
  currency?: string;
  /** "1.2K" instead of "1,200". */
  compact?: boolean;
  maximumFractionDigits?: number;
  className?: string;
}

/** A number, percentage or amount of money formatted with Intl for the active locale. */
export function NumberFormat({ value, format = 'decimal', currency, compact = false, maximumFractionDigits, className }: NumberFormatProps) {
  const intl = useIntl();
  const notation = compact ? 'compact' : 'standard';
  const text = intl.formatNumber(value, { style: format, currency, notation, maximumFractionDigits });
  return <span className={className}>{text}</span>;
}

const DURATION_UNITS = [
  ['day', 86_400_000],
  ['hour', 3_600_000],
  ['minute', 60_000],
  ['second', 1000],
  ['millisecond', 1],
] as const;

/**
 * Formats a length of time such as "2 hr, 5 min" with Intl number and list
 * formats. At most `parts` units are shown, starting at the largest non-zero one.
 */
export function formatDuration(ms: number, locale: string, parts = 2): string {
  if (ms <= 0) return new Intl.NumberFormat(locale, { style: 'unit', unit: 'second', unitDisplay: 'short' }).format(0);
  let rest = Math.round(ms);
  const pieces: string[] = [];
  for (const [unit, size] of DURATION_UNITS) {
    const amount = Math.floor(rest / size);
    if (amount > 0 && pieces.length < parts) {
      pieces.push(new Intl.NumberFormat(locale, { style: 'unit', unit, unitDisplay: 'short' }).format(amount));
    }
    rest -= amount * size;
    if (pieces.length >= parts) break;
  }
  return new Intl.ListFormat(locale, { style: 'narrow', type: 'unit' }).format(pieces);
}

/** A duration in milliseconds, formatted with Intl. */
export function Duration({ ms, parts = 2, className }: { ms: number; parts?: number; className?: string }) {
  const intl = useIntl();
  return <span className={className}>{formatDuration(ms, intl.locale, parts)}</span>;
}
