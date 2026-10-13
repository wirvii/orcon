// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'action', 'mascot'] as const;

export const pairs = [
  pair('fg.default', 'bg.app'),
  pair('fg.default', 'surface.default'),
  pair('fg.muted', 'bg.app'),
  pair('fg.muted', 'surface.default'),
] as const;
