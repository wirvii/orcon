// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'description', 'required', 'error', 'group', 'label-action'] as const;

export const pairs = [
  pair('fg.default', 'surface.default'),
  pair('fg.default', 'bg.app'),
  pair('fg.muted', 'surface.default'),
  pair('fg.muted', 'bg.app'),
  pair('status.danger.fg', 'surface.default'),
  pair('status.danger.fg', 'bg.app'),
] as const;
