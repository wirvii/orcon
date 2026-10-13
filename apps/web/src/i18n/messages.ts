// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import en from './en.json';
import es from './es.json';

/**
 * The messages of the app on top of the design system's, per product locale. `en.json` is
 * generated from the message descriptors in the code (`pnpm i18n:write`); `es.json` is
 * translated by hand. `pnpm i18n:check` fails when they drift.
 */
export const appMessages = { en, es };
