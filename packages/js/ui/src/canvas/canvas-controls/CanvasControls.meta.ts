// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'zoomed', 'with-tidy-up', 'hover', 'focus-visible'] as const;

export const pairs = [pair('fg.default', 'surface.raised'), pair('border.control', 'surface.raised')] as const;
