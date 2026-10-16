// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createFileRoute } from '@tanstack/react-router';

import { RouteError } from '../../../app/errors/RouteError';
import { SecurityScreen } from '../screens/SecurityScreen';

/** Password and sessions. */
export const Route = createFileRoute('/_session/orgs/$orgSlug/settings/security')({
  errorComponent: RouteError,
  component: SecurityScreen,
});
