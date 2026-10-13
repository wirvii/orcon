// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../lib/meta';

export const states = ['default', 'empty', 'view-only', 'execution', 'details'] as const;

export const pairs = [
  pair('fg.default', 'bg.app'),
  pair('fg.muted', 'bg.app'),
  pair('fg.default', 'surface.raised'),
  pair('fg.muted', 'surface.raised'),
  pair('fg.link', 'bg.app'),
  pair('canvas.edge', 'canvas.bg'),
  pair('canvas.edge-active', 'canvas.bg'),
  pair('canvas.port', 'canvas.bg'),
  pair('canvas.selection', 'canvas.bg'),
] as const;
