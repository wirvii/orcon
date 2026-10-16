// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'hover', 'focus-visible', 'with-trailing', 'long-title'] as const;

export const pairs = [
  pair('fg.default', 'surface.default'),
  pair('fg.default', 'surface.hover'),
  pair('fg.muted', 'surface.default'),
  pair('fg.muted', 'surface.hover'),
  pair('focus.ring', 'surface.default'),
] as const;
