// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0
//
// Template for features/<area>/routes/<name>.tsx (routing.md RT-1 to RT-7).
// Route files stay thin: parse params and search, prefetch through Query, and
// render the screen.

import { createFileRoute } from '@tanstack/react-router';
import { z } from 'zod';
import { RouteError } from '../../../app/errors/RouteError';
import { widgetQueries } from '../api/queries';
import { WidgetScreen } from '../screens/WidgetScreen';

const paramsSchema = z.object({ widgetId: z.uuid() });

// Every search field has a default or .catch(), so a malformed URL falls back.
const searchSchema = z.object({
  tab: z.enum(['overview', 'history']).catch('overview'),
});

export const Route = createFileRoute('/widgets/$widgetId')({
  params: { parse: (raw) => paramsSchema.parse(raw) },
  validateSearch: searchSchema,
  loaderDeps: ({ search }) => ({ tab: search.tab }),
  loader: ({ context, params }) =>
    context.queryClient.ensureQueryData(widgetQueries.detail(context.orgId, params.widgetId)),
  errorComponent: RouteError,
  component: WidgetScreen,
});
