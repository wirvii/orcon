// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'json', 'schema', 'input-empty', 'output-empty', 'loading', 'error-state', 'pinned', 'multi-output', 'protected'] as const;

export const pairs = [
  pair('fg.default', 'surface.default'),
  pair('fg.muted', 'surface.default'),
  pair('fg.default', 'surface.sunken'),
  pair('fg.default', 'bg.app'),
  pair('status.neutral.fg', 'status.neutral.bg'),
  pair('status.danger.fg', 'status.danger.bg'),
] as const;
