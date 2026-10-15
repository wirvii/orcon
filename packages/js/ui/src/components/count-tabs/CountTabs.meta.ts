// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'hover', 'focus-visible', 'current', 'with-counts'] as const;

export const pairs = [
  pair('fg.subtle', 'bg.app'),
  pair('fg.subtle', 'surface.default'),
  pair('fg.default', 'bg.app'),
  pair('fg.default', 'surface.default'),
  pair('status.neutral.fg', 'status.neutral.bg'),
  pair('canvas.selection', 'surface.default'),
  pair('focus.ring', 'surface.default'),
] as const;
