// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createFileRoute } from '@tanstack/react-router';

import { RouteError } from '../../../app/errors/RouteError';
import { executionsSearchSchema } from '../model/params';
import { OrgExecutionsScreen } from '../screens/ExecutionsScreen';

/**
 * The executions of the whole organization: the list with its filters in the address, and the open run beside it
 * (SPEC-019 UX7). The server lists the runs the person can read, whatever the project (`listOrgExecutions`).
 */
export const Route = createFileRoute('/_session/orgs/$orgSlug/executions')({
  validateSearch: executionsSearchSchema,
  errorComponent: RouteError,
  component: OrgExecutionsScreen,
});
