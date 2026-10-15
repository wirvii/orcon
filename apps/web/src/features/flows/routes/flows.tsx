// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createFileRoute } from '@tanstack/react-router';
import { z } from 'zod';

import { RouteError } from '../../../app/errors/RouteError';
import { FlowListScreen } from '../screens/FlowListScreen';

/** `create` opens the new flow dialog on arrival, so the sidebar and the shortcut N can lead straight to it. A malformed value is no value. */
const searchSchema = z.object({ create: z.boolean().optional().catch(undefined) });

/** The flows of the project. */
export const Route = createFileRoute('/_session/orgs/$orgSlug/projects/$projectHandle/flows')({
  validateSearch: searchSchema,
  errorComponent: RouteError,
  component: FlowListScreen,
});
