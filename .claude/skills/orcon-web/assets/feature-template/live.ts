// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0
//
// Template for features/<area>/api/live.ts (data-and-live-updates.md DT-12 to
// DT-15). Each handler patches a cached resource when the event carries
// enough, or invalidates the narrowest keys. The dispatcher in app/live-events
// parses events, coalesces invalidations and handles stream.reset.

import type { QueryClient } from '@tanstack/react-query';
import type { WidgetDeletedEvent, WidgetUpdatedEvent } from '@orcon/sdk';
import type { LiveHandlers } from '../../../app/live-events/dispatcher';
import { widgetKeys } from './queries';

export const widgetLiveHandlers = {
  'widget.updated': (event: WidgetUpdatedEvent, queryClient: QueryClient) => {
    queryClient.setQueryData(widgetKeys.detail(event.orgId, event.data.id), event.data);
    return [widgetKeys.lists(event.orgId)];
  },
  'widget.deleted': (event: WidgetDeletedEvent, queryClient: QueryClient) => {
    queryClient.removeQueries({ queryKey: widgetKeys.detail(event.orgId, event.data.id) });
    return [widgetKeys.lists(event.orgId)];
  },
} satisfies LiveHandlers;
