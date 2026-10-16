// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'hover', 'focus-visible', 'checked', 'busy', 'blocked'] as const;

export const pairs = [
  pair('fg.muted', 'surface.default'),
  pair('fg.muted', 'surface.sunken'),
  pair('status.success.fg', 'surface.default'),
  pair('border.control', 'surface.default'),
  pair('border.control', 'surface.sunken'),
  pair('action.primary.fg', 'action.primary.bg'),
  pair('action.primary.bg', 'surface.default'),
  pair('focus.ring', 'surface.default'),
] as const;
