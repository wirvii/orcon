// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'input', 'output', 'selected', 'connecting-valid'] as const;

export const pairs = [pair('canvas.port', 'canvas.node.bg'), pair('canvas.selection', 'canvas.bg'), pair('action.primary.bg', 'bg.app')] as const;
