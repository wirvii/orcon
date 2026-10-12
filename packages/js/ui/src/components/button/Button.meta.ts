// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'hover', 'focus-visible', 'active', 'disabled', 'loading'] as const;

export const pairs = [
  pair('action.primary.fg', 'action.primary.bg'),
  pair('action.primary.fg', 'action.primary.bg-hover'),
  pair('action.danger.fg', 'action.danger.bg'),
  pair('action.danger.fg', 'action.danger.bg-hover'),
  pair('fg.default', 'surface.default'),
  pair('fg.default', 'surface.sunken'),
  pair('fg.default', 'bg.app'),
  pair('border.control', 'surface.default'),
  pair('border.control', 'surface.sunken'),
  pair('focus.ring', 'surface.default'),
  pair('fg.disabled', 'surface.default'),
] as const;
