// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'design', 'succeeded', 'running', 'failed', 'uncertain', 'not-run', 'active', 'loop-back'] as const;

export const pairs = [
  pair('canvas.edge', 'canvas.bg'),
  pair('canvas.edge-active', 'canvas.bg'),
  pair('status.active.fg', 'canvas.bg'),
  pair('status.danger.fg', 'canvas.bg'),
  pair('status.attention.fg', 'canvas.bg'),
] as const;
