// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'hover', 'focus-visible', 'active', 'external', 'prose'] as const;

export const pairs = [
  pair('fg.link', 'bg.app'),
  pair('fg.link', 'surface.default'),
  pair('fg.link', 'surface.raised'),
  pair('fg.link', 'surface.sunken'),
] as const;
