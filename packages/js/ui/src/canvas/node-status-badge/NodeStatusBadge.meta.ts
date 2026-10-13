// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'running', 'succeeded', 'succeeded-with-errors', 'failed', 'uncertain', 'canceled', 'not-run'] as const;

export const pairs = [
  pair('status.active.bg', 'status.active.fg'),
  pair('status.success.bg', 'status.success.fg'),
  pair('status.warning.bg', 'status.warning.fg'),
  pair('status.danger.bg', 'status.danger.fg'),
  pair('status.attention.bg', 'status.attention.fg'),
  pair('status.active.fg', 'canvas.bg'),
  pair('status.success.fg', 'canvas.bg'),
  pair('status.warning.fg', 'canvas.bg'),
  pair('status.danger.fg', 'canvas.bg'),
  pair('status.attention.fg', 'canvas.bg'),
] as const;
