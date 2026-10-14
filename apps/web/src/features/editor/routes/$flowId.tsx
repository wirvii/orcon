// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createFileRoute, notFound } from '@tanstack/react-router';
import { z } from 'zod';

import { RouteError } from '../../../app/errors/RouteError';
import { EditorScreen } from '../screens/EditorScreen';

const flowIdSchema = z.uuid();

// The editor is the route the whole flow is edited in. This file stays thin: it is part of the
// initial JavaScript, so the data layer is imported in the loader, when the route is entered, and
// the screen is split out of it by the route plugin (orcon-web PF-2).
export const Route = createFileRoute('/flows/$flowId')({
  params: {
    parse: (raw) => {
      const flowId = flowIdSchema.safeParse(raw.flowId);
      // An address that is not a flow id names nothing: it is a page that does not exist.
      // eslint-disable-next-line @typescript-eslint/only-throw-error -- the router's notFound() is a marker object it catches, not an Error
      if (!flowId.success) throw notFound();
      return { flowId: flowId.data };
    },
    stringify: ({ flowId }) => ({ flowId }),
  },
  // The draft is prefetched through Query (orcon-web RT-5); the screen copies it into the editor store once.
  loader: async ({ context, params }) => {
    const { draftQueryOptions } = await import('../api/queries');
    return context.queryClient.query({ ...draftQueryOptions(params.flowId), staleTime: 'static' });
  },
  errorComponent: RouteError,
  component: EditorScreen,
});
