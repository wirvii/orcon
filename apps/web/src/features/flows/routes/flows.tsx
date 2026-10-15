// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createFileRoute } from '@tanstack/react-router';

import { RouteError } from '../../../app/errors/RouteError';
import { FlowListScreen } from '../screens/FlowListScreen';

/** The flows of the project. */
export const Route = createFileRoute('/_session/orgs/$orgSlug/projects/$projectHandle/flows')({
  errorComponent: RouteError,
  component: FlowListScreen,
});
