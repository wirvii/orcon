// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'hover', 'focus-visible', 'disabled', 'error', 'with-icon', 'with-adornment'] as const;

export const pairs = [
  pair('fg.default', 'surface.default'),
  pair('fg.subtle', 'surface.default'),
  pair('border.control', 'surface.default'),
  pair('fg.muted', 'surface.default'),
  pair('status.danger.fg', 'surface.default'),
  pair('fg.disabled', 'surface.default'),
  pair('focus.ring', 'surface.default'),
] as const;
