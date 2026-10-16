// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'custom-icon'] as const;

export const pairs = [
  pair('fg.default', 'surface.default'),
  pair('fg.muted', 'surface.default'),
  pair('status.neutral.fg', 'status.neutral.bg'),
] as const;
