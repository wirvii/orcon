// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'date-only', 'time-zone'] as const;

export const pairs = [pair('fg.default', 'bg.app'), pair('fg.default', 'surface.default'), pair('fg.muted', 'surface.default')] as const;
