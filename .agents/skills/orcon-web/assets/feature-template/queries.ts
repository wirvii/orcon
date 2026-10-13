// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0
//
// Template for features/<area>/api/queries.ts (data-and-live-updates.md DT-3
// to DT-5, DT-10). Replace `widget` with the area's resource. Every key starts
// with the organization id; components and loaders use these options only.

import { infiniteQueryOptions, queryOptions } from '@tanstack/react-query';
import { widgetSchema, widgetPageSchema } from '@orcon/api-contract/zod';
import type { OrgId, WidgetId, WidgetFilters } from '@orcon/sdk';
import { sdk } from '@orcon/sdk';
import { parseWith } from '../../../app/boundary/parse';

export const widgetKeys = {
  all: (org: OrgId) => [org, 'widgets'] as const,
  lists: (org: OrgId) => [...widgetKeys.all(org), 'list'] as const,
  list: (org: OrgId, filters: WidgetFilters) => [...widgetKeys.lists(org), filters] as const,
  details: (org: OrgId) => [...widgetKeys.all(org), 'detail'] as const,
  detail: (org: OrgId, id: WidgetId) => [...widgetKeys.details(org), id] as const,
};

export const widgetQueries = {
  // Cursor pagination with pageToken; at most 200 items per page (DT-10, PF-7).
  list: (org: OrgId, filters: WidgetFilters) =>
    infiniteQueryOptions({
      queryKey: widgetKeys.list(org, filters),
      initialPageParam: undefined as string | undefined,
      queryFn: async ({ pageParam, signal }) =>
        parseWith(
          widgetPageSchema,
          await sdk.widgets.list({ org, ...filters, pageToken: pageParam, limit: 50 }, { signal }),
        ),
      getNextPageParam: (page) => page.nextPageToken ?? undefined,
    }),

  // A parse failure is a contract error (errors.md ER-5).
  detail: (org: OrgId, id: WidgetId) =>
    queryOptions({
      queryKey: widgetKeys.detail(org, id),
      queryFn: async ({ signal }) => parseWith(widgetSchema, await sdk.widgets.get({ org, id }, { signal })),
    }),
};
