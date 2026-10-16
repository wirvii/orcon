// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createFileRoute } from '@tanstack/react-router';

import { redirectLegacySettings } from '../api/legacy-settings';

/** The old address of Settings alone. It only forwards (see `redirectLegacySettings`). */
export const Route = createFileRoute('/settings')({
  beforeLoad: ({ context }) => redirectLegacySettings(context.queryClient, 'profile'),
});
