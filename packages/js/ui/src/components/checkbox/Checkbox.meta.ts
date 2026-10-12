// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'hover', 'focus-visible', 'checked', 'indeterminate', 'disabled', 'error'] as const;

export const pairs = [
  pair('fg.default', 'surface.default'),
  pair('fg.default', 'surface.sunken'),
  pair('border.control', 'surface.default'),
  pair('action.primary.fg', 'action.primary.bg'),
  pair('action.primary.bg', 'surface.default'),
  pair('status.danger.fg', 'surface.default'),
  pair('fg.disabled', 'surface.default'),
  pair('focus.ring', 'surface.default'),
] as const;
