// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'success', 'warning', 'danger', 'attention', 'action'] as const;

export const pairs = [
  pair('status.neutral.fg', 'status.neutral.bg'),
  pair('status.success.fg', 'status.success.bg'),
  pair('status.warning.fg', 'status.warning.bg'),
  pair('status.danger.fg', 'status.danger.bg'),
  pair('status.attention.fg', 'status.attention.bg'),
] as const;
