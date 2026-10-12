// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../lib/meta';

export const states = ['default', 'one-item', 'loop-back'] as const;

export const pairs = [pair('fg.muted', 'surface.raised')] as const;
