// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'success', 'warning', 'danger', 'action'] as const;

export const pairs = [
  pair('fg.default', 'surface.raised'),
  pair('fg.muted', 'surface.raised'),
  pair('status.success.fg', 'surface.raised'),
  pair('status.warning.fg', 'surface.raised'),
  pair('status.danger.fg', 'surface.raised'),
  pair('border.default', 'surface.default'),
] as const;
