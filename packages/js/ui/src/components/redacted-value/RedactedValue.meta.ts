// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'visible'] as const;

export const pairs = [pair('fg.muted', 'surface.sunken'), pair('fg.default', 'surface.sunken')] as const;
