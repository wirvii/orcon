// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'welcome', 'happy', 'thinking', 'bare'] as const;

export const pairs = [pair('fg.default', 'mascot.panel')] as const;
