// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { setupServer } from 'msw/node';

/**
 * The Mock Service Worker server of the `screens` project. It starts with no handlers: each
 * test adds the ones it needs with `server.use(...)`, from the handlers generated from the API
 * contract (`@orcon/api-contract/msw`) once the real contract exists.
 */
export const server = setupServer();
