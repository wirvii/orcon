// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'open', 'footer'] as const;

export const pairs = [
  pair('fg.default', 'surface.raised'),
  pair('fg.muted', 'surface.raised'),
  pair('border.control', 'surface.raised'),
  pair('focus.ring', 'surface.raised'),
] as const;
