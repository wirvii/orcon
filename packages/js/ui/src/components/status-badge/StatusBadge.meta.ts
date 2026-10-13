// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

// "default" is QUEUED; the other states are the remaining entries of status-map.ts.
export const states = [
  'default',
  'running',
  'waiting',
  'suspended',
  'canceling',
  'canceled',
  'compensating',
  'succeeded',
  'succeeded-with-errors',
  'failed',
  'uncertain',
  'retry-scheduled',
  'not-dispatched',
  'not-run',
  'awaiting-person',
] as const;

export const pairs = [
  pair('status.neutral.fg', 'status.neutral.bg'),
  pair('status.active.fg', 'status.active.bg'),
  pair('status.success.fg', 'status.success.bg'),
  pair('status.warning.fg', 'status.warning.bg'),
  pair('status.danger.fg', 'status.danger.bg'),
  pair('status.attention.fg', 'status.attention.bg'),
] as const;
