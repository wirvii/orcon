// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'hover', 'focus-visible'] as const;

export const pairs = [pair('fg.default', 'canvas.bg'), pair('fg.muted', 'surface.default'), pair('canvas.edge', 'canvas.bg')] as const;
