// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createFileRoute } from '@tanstack/react-router';

import { RouteError } from '../../../app/errors/RouteError';
import { AppearanceScreen } from '../screens/AppearanceScreen';

/** Theme, density and language. They apply at once and are kept in this browser. */
export const Route = createFileRoute('/_session/orgs/$orgSlug/settings/appearance')({
  errorComponent: RouteError,
  component: AppearanceScreen,
});
