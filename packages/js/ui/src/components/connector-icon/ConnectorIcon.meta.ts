// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'tile', 'fallback', 'dark-variant'] as const;

export const pairs = [pair('border.default', 'surface.sunken'), pair('fg.muted', 'surface.sunken')] as const;
