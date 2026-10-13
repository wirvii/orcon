// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../lib/meta';

export const states = ['default', 'empty', 'view-only', 'outline-open', 'execution', 'agent', 'compare', 'performance', 'keyboard'] as const;

export const pairs = [
  pair('fg.default', 'surface.raised'),
  pair('fg.muted', 'surface.raised'),
  pair('fg.default', 'surface.default'),
  pair('fg.muted', 'surface.default'),
  pair('fg.muted', 'bg.app'),
  pair('fg.default', 'canvas.bg'),
  pair('fg.muted', 'canvas.bg'),
  pair('canvas.edge', 'canvas.bg'),
  pair('canvas.edge-active', 'canvas.bg'),
  pair('canvas.port', 'canvas.bg'),
  pair('canvas.selection', 'canvas.bg'),
] as const;
