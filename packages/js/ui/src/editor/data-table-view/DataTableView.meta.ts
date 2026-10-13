// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'nested', 'protected', 'binary', 'empty', 'virtualized'] as const;

export const pairs = [
  pair('fg.default', 'surface.default'),
  pair('fg.muted', 'surface.sunken'),
  pair('fg.subtle', 'surface.default'),
  pair('fg.subtle', 'surface.sunken'),
  pair('fg.link', 'surface.default'),
  pair('fg.muted', 'surface.default'),
] as const;
