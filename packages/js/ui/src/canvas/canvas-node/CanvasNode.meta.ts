// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'trigger', 'logic', 'agent', 'placeholder', 'selected', 'hover', 'focus-visible', 'running', 'succeeded', 'succeeded-with-errors', 'failed', 'uncertain', 'waiting', 'canceled', 'not-run', 'disabled', 'pinned', 'compare'] as const;

export const pairs = [
  pair('fg.default', 'surface.default'),
  pair('fg.muted', 'canvas.bg'),
  pair('fg.default', 'canvas.bg'),
  pair('fg.accent', 'surface.default'),
  pair('fg.link', 'surface.default'),
  pair('fg.link', 'status.danger.bg'),
  pair('fg.accent', 'status.danger.bg'),
  pair('canvas.port', 'canvas.bg'),
  pair('canvas.selection', 'canvas.bg'),
  pair('canvas.edge', 'surface.sunken'),
  pair('status.active.fg', 'canvas.bg'),
  pair('status.warning.fg', 'canvas.bg'),
  pair('status.danger.fg', 'canvas.bg'),
  pair('status.attention.fg', 'canvas.bg'),
  pair('status.success.fg', 'canvas.bg'),
] as const;
