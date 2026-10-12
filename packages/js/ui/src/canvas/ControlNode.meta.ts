// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../lib/meta';

export const states = ['default', 'if', 'switch', 'merge', 'loop', 'wait', 'selected'] as const;

export const pairs = [
  pair('fg.default', 'surface.raised'),
  pair('fg.muted', 'surface.raised'),
  pair('canvas.port', 'canvas.bg'),
  pair('canvas.selection', 'canvas.bg'),
] as const;
