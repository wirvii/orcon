// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'collapsed', 'protected', 'focus-visible'] as const;

export const pairs = [
  pair('fg.default', 'surface.default'),
  pair('fg.accent', 'surface.default'),
  pair('fg.link', 'surface.default'),
  pair('fg.subtle', 'surface.default'),
  pair('fg.muted', 'surface.default'),
  pair('fg.default', 'surface.sunken'),
] as const;
