// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'node', 'raw'] as const;

export const pairs = [pair('fg.accent', 'surface.sunken'), pair('fg.default', 'surface.sunken'), pair('fg.muted', 'surface.sunken')] as const;
