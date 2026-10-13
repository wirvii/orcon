// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'single', 'long'] as const;

export const pairs = [
  pair('fg.link', 'bg.app'),
  pair('fg.default', 'bg.app'),
  pair('fg.muted', 'bg.app'),
] as const;
