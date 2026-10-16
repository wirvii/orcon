// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createFileRoute } from '@tanstack/react-router';

import { RouteError } from '../../../app/errors/RouteError';
import { ApiKeysScreen } from '../screens/ApiKeysScreen';

/** API keys: not available until the server creates them. */
export const Route = createFileRoute('/_session/orgs/$orgSlug/settings/api-keys')({
  errorComponent: RouteError,
  component: ApiKeysScreen,
});
