// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'trigger', 'disabled-node', 'hover', 'focus-visible', 'unsupported'] as const;

export const pairs = [pair('fg.default', 'surface.raised'), pair('fg.accent', 'surface.raised'), pair('status.danger.fg', 'status.danger.bg')] as const;
