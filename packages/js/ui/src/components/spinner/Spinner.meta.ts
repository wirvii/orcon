// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default'] as const;

export const pairs = [pair('fg.muted', 'surface.default'), pair('fg.muted', 'bg.app')] as const;
