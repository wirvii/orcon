// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'hover', 'selected', 'sticky-header'] as const;

export const pairs = [
  pair('fg.default', 'surface.default'),
  pair('fg.default', 'surface.sunken'),
  pair('fg.muted', 'surface.default'),
  pair('border.default', 'surface.default'),
  pair('action.primary.bg', 'surface.sunken'),
] as const;
