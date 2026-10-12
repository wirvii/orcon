// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../lib/meta';

export const states = ['default', 'empty', 'error', 'batch-selected'] as const;

export const pairs = [
  pair('fg.default', 'bg.app'),
  pair('fg.muted', 'bg.app'),
  pair('fg.default', 'surface.default'),
  pair('fg.muted', 'surface.default'),
  pair('fg.default', 'surface.sunken'),
  pair('fg.muted', 'surface.sunken'),
  pair('fg.link', 'surface.default'),
  pair('status.attention.fg', 'surface.default'),
  pair('status.success.fg', 'surface.default'),
  pair('status.danger.fg', 'surface.default'),
  pair('status.attention.fg', 'status.attention.bg'),
  pair('status.danger.fg', 'status.danger.bg'),
  pair('border.control', 'surface.default'),
] as const;
