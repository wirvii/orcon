// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createFileRoute } from '@tanstack/react-router';

import { RouteError } from '../../../app/errors/RouteError';
import { ProfileScreen } from '../screens/ProfileScreen';

/** The profile of the signed-in person. */
export const Route = createFileRoute('/_session/orgs/$orgSlug/settings/profile')({
  errorComponent: RouteError,
  component: ProfileScreen,
});
