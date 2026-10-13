// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { QueryClient } from '@tanstack/react-query';

/** What every route loader receives as `context`: loaders prefetch through the query client (orcon-web DT-11). */
export interface RouterContext {
  readonly queryClient: QueryClient;
}
