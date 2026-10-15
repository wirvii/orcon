// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createFileRoute } from '@tanstack/react-router';

import { RouteError } from '../../../app/errors/RouteError';
import { appearanceSearchSchema } from '../model/preferences';
import { AppearanceScreen } from '../screens/AppearanceScreen';

/** The page is public: a person can choose the theme and language before signing in (SPEC-016 C-8). The frame comes from the settings layout. */
function AppearancePage() {
  return <AppearanceScreen />;
}

// This page reads no server data, so it has no loader. A route that does prefetches through
// Query: `loader: ({ context }) => context.queryClient.ensureQueryData(...)` (orcon-web RT-5).
export const Route = createFileRoute('/settings/appearance')({
  validateSearch: appearanceSearchSchema,
  errorComponent: RouteError,
  component: AppearancePage,
});
