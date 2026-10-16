// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'title-only', 'with-fix'] as const;

export const pairs = [
  pair('status.danger.fg', 'status.danger.bg'),
  pair('fg.default', 'surface.default'),
  pair('fg.default', 'surface.sunken'),
] as const;
