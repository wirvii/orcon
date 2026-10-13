// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { queryOptions } from '@tanstack/react-query';

import { parseWith } from '../../../app/boundary/parse';
import { flowSchema } from '../model/flow';

export const flowKeys = {
  all: (org: string) => [org, 'flows'] as const,
  detail: (org: string, id: string) => [...flowKeys.all(org), 'detail', id] as const,
};

export const flowQueries = {
  detail: (org: string, id: string) =>
    queryOptions({
      queryKey: flowKeys.detail(org, id),
      queryFn: () => Promise.resolve(parseWith(flowSchema, { id, name: org })),
    }),
};
