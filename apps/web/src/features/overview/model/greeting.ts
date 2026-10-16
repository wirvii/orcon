// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

export type Greeting = 'morning' | 'afternoon' | 'evening';

/** Which greeting fits an hour of the day (0 to 23): morning until noon, afternoon until 6 pm, evening after. */
export function greetingFor(hour: number): Greeting {
  if (hour < 12) return 'morning';
  if (hour < 18) return 'afternoon';
  return 'evening';
}
