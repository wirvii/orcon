// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'focus-visible', 'single'] as const;

export const pairs = [pair('status.danger.fg', 'status.danger.bg')] as const;
