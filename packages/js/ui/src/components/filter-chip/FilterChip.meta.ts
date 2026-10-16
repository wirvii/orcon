// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'hover', 'focus-visible', 'pressed', 'menu', 'disabled'] as const;

export const pairs = [
  pair('fg.muted', 'surface.default'),
  pair('fg.default', 'surface.hover'),
  pair('border.control', 'surface.default'),
  pair('fg.disabled', 'surface.default'),
  pair('focus.ring', 'surface.default'),
] as const;
