// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'triggers-only', 'no-match', 'focus-visible'] as const;

export const pairs = [pair('fg.default', 'surface.raised'), pair('fg.muted', 'surface.raised'), pair('fg.subtle', 'surface.raised')] as const;
