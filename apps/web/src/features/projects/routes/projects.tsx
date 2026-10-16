// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createFileRoute } from '@tanstack/react-router';
import { z } from 'zod';

import { RouteError } from '../../../app/errors/RouteError';
import { ProjectListScreen } from '../screens/ProjectListScreen';

/** `create` opens the new project dialog on arrival, so the sidebar can lead straight to it. A malformed value is no value. */
const searchSchema = z.object({ create: z.boolean().optional().catch(undefined) });

/** The projects of the organization the person can read. */
export const Route = createFileRoute('/_session/orgs/$orgSlug/projects')({
  validateSearch: searchSchema,
  errorComponent: RouteError,
  component: ProjectListScreen,
});
