// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createFileRoute, notFound } from '@tanstack/react-router';
import { z } from 'zod';

import { RouteError } from '../../../app/errors/RouteError';
import { ExecutionDetailScreen } from '../screens/ExecutionDetailScreen';

const executionIdSchema = z.uuid();

/** One run of the project. An address that is not a run id names nothing: it is a page that does not exist. */
export const Route = createFileRoute('/_session/orgs/$orgSlug/projects/$projectHandle/executions/$executionId')({
  params: {
    parse: (raw) => {
      const id = executionIdSchema.safeParse(raw.executionId);
      // eslint-disable-next-line @typescript-eslint/only-throw-error -- the router's notFound() is a marker object it catches, not an Error
      if (!id.success) throw notFound();
      return { executionId: id.data };
    },
    stringify: ({ executionId }) => ({ executionId }),
  },
  errorComponent: RouteError,
  component: ExecutionDetailScreen,
});
