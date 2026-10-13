// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'selected', 'acknowledged', 'escalated', 'overdue'] as const;

export const pairs = [
  pair('fg.default', 'surface.default'),
  pair('fg.muted', 'surface.default'),
  pair('fg.default', 'surface.sunken'),
  pair('fg.muted', 'surface.sunken'),
  pair('fg.link', 'surface.default'),
  pair('fg.link', 'surface.sunken'),
  pair('status.attention.fg', 'status.attention.bg'),
  pair('status.warning.fg', 'status.warning.bg'),
  pair('status.danger.fg', 'status.danger.bg'),
  pair('border.default', 'surface.default'),
] as const;
