// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { useIntl } from 'react-intl';

import { cn } from '../../lib/cn';

export type DateInput = Date | string | number;

export function toDate(value: DateInput): Date {
  return value instanceof Date ? value : new Date(value);
}

export interface DateTimeProps {
  value: DateInput;
  /** Date style. Defaults to medium. */
  dateStyle?: 'full' | 'long' | 'medium' | 'short';
  /** Adds the time of day after the date. */
  timeStyle?: 'long' | 'medium' | 'short';
  /** Show only the time of day. */
  timeOnly?: boolean;
  /** Show the time zone name. Use when the reader could confuse zones. Spells the date out as day, month and year. */
  showTimeZone?: boolean;
  className?: string;
}

/**
 * A date and time formatted with Intl for the active locale, in the browser time
 * zone. The machine-readable ISO timestamp is in the `datetime` attribute and in
 * the hover title.
 */
export function DateTime({ value, dateStyle = 'medium', timeStyle, timeOnly = false, showTimeZone = false, className }: DateTimeProps) {
  const intl = useIntl();
  const date = toDate(value);
  if (Number.isNaN(date.getTime())) return null;

  let options: Intl.DateTimeFormatOptions;
  if (showTimeZone) {
    // timeZoneName cannot be combined with dateStyle or timeStyle, so the parts are spelled out.
    options = {
      ...(timeOnly ? {} : { year: 'numeric', month: 'short', day: 'numeric' }),
      ...(timeStyle || timeOnly ? { hour: 'numeric', minute: '2-digit' } : {}),
      timeZoneName: 'short',
    };
  } else if (timeOnly) {
    options = { timeStyle: timeStyle ?? 'short' };
  } else {
    options = { dateStyle, timeStyle };
  }
  const text = intl.formatDate(date, options);

  return (
    <time dateTime={date.toISOString()} title={date.toISOString()} className={cn('whitespace-nowrap', className)}>
      {text}
    </time>
  );
}
