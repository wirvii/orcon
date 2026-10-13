// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'succeeded', 'running', 'failed', 'uncertain', 'pinned', 'loop-back'] as const;

export const pairs = [
  pair('status.success.fg', 'surface.default'),
  pair('status.active.fg', 'surface.default'),
  pair('status.danger.fg', 'surface.default'),
  pair('status.attention.fg', 'surface.default'),
  pair('fg.muted', 'surface.default'),
] as const;
