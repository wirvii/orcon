// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../lib/meta';

export const states = ['default', 'input', 'output', 'selected'] as const;

export const pairs = [pair('canvas.port', 'canvas.bg'), pair('canvas.selection', 'canvas.bg')] as const;
