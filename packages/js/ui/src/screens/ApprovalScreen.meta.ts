// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../lib/meta';

export const states = ['default', 'redacted', 'denying', 'editing', 'invalid-arguments', 'expired', 'already-resolved'] as const;

export const pairs = [
  pair('fg.default', 'bg.app'),
  pair('fg.muted', 'bg.app'),
  pair('fg.default', 'surface.default'),
  pair('fg.muted', 'surface.default'),
  pair('fg.default', 'surface.sunken'),
  pair('status.danger.fg', 'surface.default'),
  pair('status.danger.fg', 'status.danger.bg'),
  pair('status.warning.fg', 'status.warning.bg'),
  pair('status.success.fg', 'status.success.bg'),
  pair('action.primary.fg', 'action.primary.bg'),
  pair('action.danger.fg', 'action.danger.bg'),
  pair('border.control', 'surface.default'),
  pair('fg.disabled', 'surface.default'),
] as const;
