// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../lib/meta';

export const states = ['default', 'running', 'succeeded', 'succeeded-with-errors', 'failed', 'uncertain', 'not-run', 'with-uncertain-items'] as const;

export const pairs = [
  pair('status.neutral.fg', 'surface.default'),
  pair('status.active.fg', 'surface.default'),
  pair('status.success.fg', 'surface.default'),
  pair('status.warning.fg', 'surface.default'),
  pair('status.danger.fg', 'surface.default'),
  pair('status.attention.fg', 'surface.default'),
] as const;
