// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../lib/meta';

export const states = ['default', 'redacted', 'no-decision', 'succeeded'] as const;

export const pairs = [
  pair('fg.default', 'bg.app'),
  pair('fg.muted', 'bg.app'),
  pair('fg.default', 'surface.default'),
  pair('fg.muted', 'surface.default'),
  pair('fg.default', 'surface.sunken'),
  pair('fg.muted', 'surface.sunken'),
  pair('status.attention.fg', 'surface.default'),
  pair('status.neutral.fg', 'status.neutral.bg'),
  pair('status.active.fg', 'status.active.bg'),
  pair('status.success.fg', 'status.success.bg'),
  pair('status.warning.fg', 'status.warning.bg'),
  pair('status.danger.fg', 'status.danger.bg'),
  pair('status.attention.fg', 'status.attention.bg'),
  pair('action.primary.fg', 'action.primary.bg'),
  pair('border.control', 'surface.default'),
] as const;
