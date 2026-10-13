// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'sorted', 'loading', 'empty', 'error', 'sticky-header'] as const;

export const pairs = [
  pair('fg.default', 'surface.default'),
  pair('fg.default', 'surface.sunken'),
  pair('fg.muted', 'surface.default'),
  pair('border.default', 'surface.default'),
  pair('status.danger.fg', 'surface.default'),
  pair('border.control', 'surface.default'),
] as const;
