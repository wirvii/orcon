// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createFileRoute } from '@tanstack/react-router';

import { RouteError } from '../../../app/errors/RouteError';
import { MembersScreen } from '../screens/ComingSoonSections';

/** People and roles: not available until the server lists people. */
export const Route = createFileRoute('/_session/orgs/$orgSlug/settings/members')({
  errorComponent: RouteError,
  component: MembersScreen,
});
