// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'hover', 'focus-visible', 'selected'] as const;

export const pairs = [pair('canvas.port', 'canvas.node.bg'), pair('action.primary.fg', 'action.primary.bg'), pair('canvas.selection', 'canvas.bg')] as const;
