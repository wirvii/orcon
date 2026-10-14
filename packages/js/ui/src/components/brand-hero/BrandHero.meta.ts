// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default'] as const;

export const pairs = [pair('fg.muted', 'bg.app'), pair('fg.muted', 'surface.default')] as const;
