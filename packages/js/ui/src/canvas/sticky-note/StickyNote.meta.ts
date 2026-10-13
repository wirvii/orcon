// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'selected', 'editing', 'empty', 'color-2', 'color-3', 'color-4', 'color-5', 'color-6'] as const;

export const pairs = [
  pair('fg.default', 'canvas.sticky.1.bg'),
  pair('fg.default', 'canvas.sticky.2.bg'),
  pair('fg.default', 'canvas.sticky.3.bg'),
  pair('fg.default', 'canvas.sticky.4.bg'),
  pair('fg.default', 'canvas.sticky.5.bg'),
  pair('fg.default', 'canvas.sticky.6.bg'),
  pair('canvas.sticky.1.accent', 'canvas.sticky.1.bg'),
  pair('canvas.sticky.2.accent', 'canvas.sticky.2.bg'),
  pair('canvas.sticky.3.accent', 'canvas.sticky.3.bg'),
  pair('canvas.sticky.4.accent', 'canvas.sticky.4.bg'),
  pair('canvas.sticky.5.accent', 'canvas.sticky.5.bg'),
  pair('canvas.sticky.6.accent', 'canvas.sticky.6.bg'),
] as const;
