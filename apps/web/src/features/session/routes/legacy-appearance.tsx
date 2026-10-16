// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createFileRoute } from '@tanstack/react-router';

import { redirectLegacySettings } from '../api/legacy-settings';

/** The old address of the appearance settings, public before settings moved into the organization. It only forwards. */
export const Route = createFileRoute('/settings/appearance')({
  beforeLoad: ({ context }) => redirectLegacySettings(context.queryClient, 'appearance'),
});
