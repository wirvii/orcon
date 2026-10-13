// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { useEffect, useState } from 'react';
import { useIntl } from 'react-intl';

import { cn } from '../../lib/cn';
import { toDate, type DateInput } from '../date-time/DateTime';

export interface RelativeParts {
  value: number;
  unit: Intl.RelativeTimeFormatUnit;
}

const SECOND = 1000;
const MINUTE = 60 * SECOND;
const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;

/**
 * Chooses the unit that reads best: seconds under a minute, then minutes, hours,
 * days, months and years. The value is negative in the past, positive in the future.
 */
export function relativeParts(diffMs: number): RelativeParts {
  const abs = Math.abs(diffMs);
  const sign = diffMs < 0 ? -1 : 1;
  if (abs < MINUTE) return { value: sign * Math.round(abs / SECOND) || 0, unit: 'second' };
  if (abs < HOUR) return { value: sign * Math.round(abs / MINUTE), unit: 'minute' };
  if (abs < DAY) return { value: sign * Math.round(abs / HOUR), unit: 'hour' };
  if (abs < 30 * DAY) return { value: sign * Math.round(abs / DAY), unit: 'day' };
  if (abs < 365 * DAY) return { value: sign * Math.round(abs / (30 * DAY)), unit: 'month' };
  return { value: sign * Math.round(abs / (365 * DAY)), unit: 'year' };
}

export interface RelativeTimeProps {
  value: DateInput;
  /** The instant "now" is measured from. Defaults to the clock and refreshes every 30 seconds. Pass it in tests and stories. */
  now?: DateInput | undefined;
  className?: string | undefined;
}

/** "5 minutes ago", "in 2 hours": Intl relative time, with the exact ISO timestamp on hover. */
export function RelativeTime({ value, now, className }: RelativeTimeProps) {
  const intl = useIntl();
  const [tick, setTick] = useState(() => Date.now());

  useEffect(() => {
    if (now !== undefined) return;
    const timer = window.setInterval(() => setTick(Date.now()), 30_000);
    return () => window.clearInterval(timer);
  }, [now]);

  const date = toDate(value);
  if (Number.isNaN(date.getTime())) return null;
  const reference = now !== undefined ? toDate(now).getTime() : tick;
  const { value: amount, unit } = relativeParts(date.getTime() - reference);

  const text = intl.formatRelativeTime(amount, unit, { numeric: 'auto' });

  return (
    <time dateTime={date.toISOString()} title={date.toISOString()} className={cn('whitespace-nowrap', className)}>
      {text}
    </time>
  );
}
