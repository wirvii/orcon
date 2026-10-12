// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../lib/meta';

export const states = ['default', 'active', 'selected', 'with-count', 'loop-back'] as const;

export const pairs = [
  pair('canvas.edge', 'canvas.bg'),
  pair('canvas.edge-active', 'canvas.bg'),
  pair('canvas.selection', 'canvas.bg'),
  pair('fg.muted', 'surface.raised'),
] as const;
