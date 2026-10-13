// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { queryOptions } from '@tanstack/react-query';

export function flowQuery(org: string, id: string) {
  return queryOptions({
    queryKey: [org, 'flows'],
    queryFn: () => Promise.resolve(`${org}:${id}`),
  });
}
