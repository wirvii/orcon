// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'hover', 'focus-visible', 'at-minimum'] as const;

export const pairs = [pair('canvas.selection', 'surface.default'), pair('focus.ring', 'surface.default')] as const;
