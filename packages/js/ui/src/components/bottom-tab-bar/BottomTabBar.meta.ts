// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'hover', 'focus-visible', 'current'] as const;

export const pairs = [
  pair('fg.subtle', 'surface.default'),
  pair('fg.default', 'surface.default'),
  pair('fg.accent', 'surface.default'),
  pair('focus.ring', 'surface.default'),
] as const;
