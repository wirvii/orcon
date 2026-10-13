// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'protected', 'long-example'] as const;

export const pairs = [pair('fg.default', 'surface.default'), pair('fg.muted', 'surface.default'), pair('fg.default', 'surface.sunken')] as const;
