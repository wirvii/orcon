// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'menu', 'actions'] as const;

export const pairs = [
  pair('fg.default', 'surface.default'),
  pair('border.default', 'surface.default'),
  pair('focus.ring', 'surface.default'),
] as const;
