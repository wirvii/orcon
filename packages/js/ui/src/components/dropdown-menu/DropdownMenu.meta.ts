// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'open', 'destructive', 'disabled'] as const;

export const pairs = [
  pair('fg.default', 'surface.raised'),
  pair('fg.default', 'surface.sunken'),
  pair('fg.muted', 'surface.raised'),
  pair('status.danger.fg', 'surface.raised'),
  pair('fg.disabled', 'surface.raised'),
  pair('focus.ring', 'surface.sunken'),
] as const;
