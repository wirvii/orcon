// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { pair } from '../../lib/meta';

export const states = ['default', 'thinking', 'thumbs-up', 'explorer'] as const;

export const pairs = [pair('border.default', 'surface.default')] as const;
