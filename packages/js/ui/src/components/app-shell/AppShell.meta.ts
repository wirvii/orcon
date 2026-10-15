// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

import { pairs as topBarPairs } from '../top-bar/TopBar.meta';

export const states = ['default', 'drawer', 'badge', 'product'] as const;

export const pairs = [...topBarPairs, pair('fg.subtle', 'surface.default'), pair('fg.default', 'surface.default'), pair('fg.default', 'bg.app')] as const;
