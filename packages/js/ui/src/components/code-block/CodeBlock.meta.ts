// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'copyable', 'long'] as const;

export const pairs = [
  pair('fg.default', 'surface.sunken'),
  pair('border.control', 'surface.sunken'),
  pair('focus.ring', 'surface.sunken'),
] as const;
