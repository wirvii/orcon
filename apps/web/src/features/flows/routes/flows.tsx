// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createFileRoute } from '@tanstack/react-router';

import { RouteError } from '../../../app/errors/RouteError';
import { flowsSearchSchema } from '../model/params';
import { FlowListScreen } from '../screens/FlowListScreen';

/** The flows of the project. `create` opens the chooser on arrival; the search and the filters live in the address. */
export const Route = createFileRoute('/_session/orgs/$orgSlug/projects/$projectHandle/flows')({
  validateSearch: flowsSearchSchema,
  errorComponent: RouteError,
  component: FlowListScreen,
});
