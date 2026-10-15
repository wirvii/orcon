// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'hover', 'focus-visible', 'compact', 'open', 'without-role'] as const;

export const pairs = [
  pair('fg.default', 'surface.default'),
  pair('fg.default', 'surface.hover'),
  pair('fg.default', 'surface.raised'),
  pair('fg.muted', 'surface.raised'),
  pair('fg.subtle', 'surface.default'),
  pair('fg.subtle', 'surface.raised'),
  pair('action.primary.fg', 'action.primary.bg'),
  pair('focus.ring', 'surface.default'),
  pair('focus.ring', 'surface.sunken'),
] as const;
