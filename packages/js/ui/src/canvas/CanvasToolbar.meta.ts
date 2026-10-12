// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../lib/meta';

export const states = ['default', 'can-undo', 'read-only', 'outline-open'] as const;

export const pairs = [
  pair('fg.default', 'surface.raised'),
  pair('fg.disabled', 'surface.raised'),
] as const;
