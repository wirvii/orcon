// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createFileRoute } from '@tanstack/react-router';

import { RouteError } from '../../../app/errors/RouteError';
import { OverviewScreen } from '../screens/OverviewScreen';

/** Home: the first page of an organization (SPEC-019 Q1). It lists the flows of every project the person can read. */
export const Route = createFileRoute('/_session/orgs/$orgSlug/overview')({
  errorComponent: RouteError,
  component: OverviewScreen,
});
