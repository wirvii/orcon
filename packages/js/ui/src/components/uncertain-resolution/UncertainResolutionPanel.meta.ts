// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'with-risk-ack', 'acknowledged', 'no-actions'] as const;

export const pairs = [
  pair('fg.default', 'surface.default'),
  pair('status.attention.fg', 'surface.default'),
  pair('border.control', 'surface.default'),
  pair('fg.disabled', 'surface.default'),
] as const;
