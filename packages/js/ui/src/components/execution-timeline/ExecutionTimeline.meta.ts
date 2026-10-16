// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'failed', 'needs-review', 'not-run', 'selected', 'selectable'] as const;

export const pairs = [
  pair('fg.default', 'surface.default'),
  pair('fg.muted', 'surface.default'),
  pair('fg.default', 'surface.hover'),
  pair('status.success.fg', 'surface.default'),
  pair('status.danger.fg', 'surface.default'),
  pair('status.attention.fg', 'surface.default'),
  pair('status.neutral.fg', 'surface.default'),
  pair('focus.ring', 'surface.default'),
] as const;
