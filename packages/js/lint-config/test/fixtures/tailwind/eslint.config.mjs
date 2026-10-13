// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

import { createConfig } from '../../../eslint.config.mjs';

const here = dirname(fileURLToPath(import.meta.url));

export default createConfig({ tailwindEntryPoint: join(here, 'entry.css') });
