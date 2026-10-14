// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createFileRoute } from '@tanstack/react-router';

import { RouteError } from '../../../app/errors/RouteError';
import { ProjectListScreen } from '../screens/ProjectListScreen';

/** The projects of the organization the person can read. */
export const Route = createFileRoute('/_session/orgs/$orgSlug/projects')({
  errorComponent: RouteError,
  component: ProjectListScreen,
});
