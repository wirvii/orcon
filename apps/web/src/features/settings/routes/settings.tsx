// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createFileRoute } from '@tanstack/react-router';

import { RouteError } from '../../../app/errors/RouteError';
import { SettingsScreen } from '../screens/SettingsScreen';

/** Settings live inside the organization, so the sidebar and the account menu stay (SPEC-019 Q4, R-UX-SHL-05). */
export const Route = createFileRoute('/_session/orgs/$orgSlug/settings')({
  errorComponent: RouteError,
  component: SettingsScreen,
});
