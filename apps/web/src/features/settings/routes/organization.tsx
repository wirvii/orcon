// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createFileRoute } from '@tanstack/react-router';

import { RouteError } from '../../../app/errors/RouteError';
import { OrganizationScreen } from '../screens/OrganizationScreen';

/** The organization: its name and web address. */
export const Route = createFileRoute('/_session/orgs/$orgSlug/settings/organization')({
  errorComponent: RouteError,
  component: OrganizationScreen,
});
