// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createFileRoute } from '@tanstack/react-router';

import { RouteError } from '../../../app/errors/RouteError';
import { executionsSearchSchema } from '../model/params';
import { ExecutionListScreen } from '../screens/ExecutionListScreen';

/** The runs of the project, optionally of one flow (`?flowId=`). */
export const Route = createFileRoute('/_session/orgs/$orgSlug/projects/$projectHandle/executions')({
  validateSearch: executionsSearchSchema,
  errorComponent: RouteError,
  component: ExecutionListScreen,
});
