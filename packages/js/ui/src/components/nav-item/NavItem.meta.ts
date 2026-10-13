// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'hover', 'focus-visible', 'current', 'collapsed', 'badge'] as const;

export const pairs = [
  pair('fg.muted', 'surface.default'),
  pair('fg.default', 'surface.default'),
  pair('fg.default', 'surface.sunken'),
  pair('action.primary.bg', 'surface.sunken'),
  pair('status.attention.fg', 'status.attention.bg'),
  pair('focus.ring', 'surface.default'),
] as const;
