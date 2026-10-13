// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'hidden'] as const;

export const pairs = [pair('canvas.selection', 'surface.default'), pair('fg.muted', 'surface.raised')] as const;
