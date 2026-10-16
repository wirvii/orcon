// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createFileRoute } from '@tanstack/react-router';

import { RouteError } from '../../../app/errors/RouteError';
import { executionsSearchSchema } from '../model/params';
import { ProjectExecutionsScreen } from '../screens/ExecutionsScreen';

/** The runs of the project (its Executions tab), optionally of one flow (`?flowId=`) and with the same filters as the list of the organization. */
export const Route = createFileRoute('/_session/orgs/$orgSlug/projects/$projectHandle/executions')({
  validateSearch: executionsSearchSchema,
  errorComponent: RouteError,
  component: ProjectExecutionsScreen,
});
