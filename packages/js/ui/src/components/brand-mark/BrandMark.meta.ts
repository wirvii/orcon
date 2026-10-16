// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'symbol', 'on-dark', 'on-light'] as const;

export const pairs = [pair('fg.default', 'bg.app'), pair('fg.default', 'surface.default'), pair('border.default', 'surface.default')] as const;
