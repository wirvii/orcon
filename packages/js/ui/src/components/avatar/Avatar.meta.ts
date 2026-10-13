// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'image'] as const;

export const pairs = [pair('fg.default', 'surface.sunken')] as const;
