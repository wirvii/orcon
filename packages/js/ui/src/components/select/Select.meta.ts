// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'hover', 'focus-visible', 'open', 'disabled', 'error'] as const;

export const pairs = [
  pair('fg.default', 'surface.default'),
  pair('fg.subtle', 'surface.default'),
  pair('border.control', 'surface.default'),
  pair('status.danger.fg', 'surface.default'),
  pair('fg.disabled', 'surface.default'),
  pair('focus.ring', 'surface.default'),
  pair('fg.default', 'surface.raised'),
  pair('fg.default', 'surface.sunken'),
  pair('focus.ring', 'surface.sunken'),
  pair('fg.disabled', 'surface.raised'),
] as const;
