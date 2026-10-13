// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'unset', 'hover', 'focus-visible'] as const;

export const pairs = [pair('fg.default', 'surface.default'), pair('fg.muted', 'canvas.bg')] as const;
