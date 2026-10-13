// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../lib/meta';

export const states = ['default'] as const;

export const pairs = [
  pair('fg.default', 'bg.app'),
  pair('fg.muted', 'bg.app'),
  pair('fg.link', 'bg.app'),
  pair('status.danger.fg', 'status.danger.bg'),
  pair('action.primary.fg', 'action.primary.bg'),
  pair('border.control', 'surface.default'),
] as const;
