// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../lib/meta';

export const states = [
  'default',
  'trigger',
  'selected',
  'running',
  'succeeded',
  'succeeded-with-errors',
  'failed',
  'uncertain',
  'with-uncertain-items',
  'not-run',
  'labelled-outputs',
] as const;

export const pairs = [
  pair('fg.default', 'surface.raised'),
  pair('fg.muted', 'surface.raised'),
  pair('status.neutral.fg', 'surface.raised'),
  pair('status.active.fg', 'surface.raised'),
  pair('status.success.fg', 'surface.raised'),
  pair('status.warning.fg', 'surface.raised'),
  pair('status.danger.fg', 'surface.raised'),
  pair('status.attention.fg', 'surface.raised'),
  pair('canvas.port', 'canvas.bg'),
  pair('canvas.selection', 'canvas.bg'),
] as const;
