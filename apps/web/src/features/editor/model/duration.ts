// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * The durations of a flow document are ISO 8601 with weeks, days, hours, minutes and seconds only
 * (docs/reference/orcon-flow-1.md "Patterns and limits"); years and months are not allowed because
 * their length varies. The editor shows them as a number of seconds and writes them back in the
 * shortest form.
 */

const DURATION = /^P(?:(\d+)W|(?:(\d+)D)?(?:T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?)?)$/;

/** The length of a duration in seconds, or undefined when the text is not a valid duration. */
export function parseDuration(text: string): number | undefined {
  const match = DURATION.exec(text);
  if (!match || text === 'P' || text.endsWith('T')) return undefined;
  const [, weeks, days, hours, minutes, seconds] = match;
  const number = (part: string | undefined) => (part === undefined ? 0 : Number(part));
  return number(weeks) * 604_800 + number(days) * 86_400 + number(hours) * 3_600 + number(minutes) * 60 + number(seconds);
}

/** A whole, positive number of seconds as a duration: `90` is `PT1M30S`, `86400` is `P1D`. */
export function formatDuration(totalSeconds: number): string {
  const total = Math.max(1, Math.round(totalSeconds));
  const days = Math.floor(total / 86_400);
  const hours = Math.floor((total % 86_400) / 3_600);
  const minutes = Math.floor((total % 3_600) / 60);
  const seconds = total % 60;
  const time = `${hours > 0 ? `${String(hours)}H` : ''}${minutes > 0 ? `${String(minutes)}M` : ''}${seconds > 0 ? `${String(seconds)}S` : ''}`;
  return `P${days > 0 ? `${String(days)}D` : ''}${time === '' ? '' : `T${time}`}`;
}
