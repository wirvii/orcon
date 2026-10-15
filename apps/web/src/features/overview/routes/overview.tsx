// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createFileRoute } from '@tanstack/react-router';

import { RouteError } from '../../../app/errors/RouteError';
import { overviewSearchSchema } from '../model/params';
import { OverviewScreen } from '../screens/OverviewScreen';

/**
 * Home: the first page of an organization (SPEC-019 Q1). The tab, the search and the filters live in the address.
 */
export const Route = createFileRoute('/_session/orgs/$orgSlug/overview')({
  validateSearch: overviewSearchSchema,
  errorComponent: RouteError,
  component: OverviewScreen,
});
