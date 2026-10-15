// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createFileRoute } from '@tanstack/react-router';

import { RouteError } from '../../../app/errors/RouteError';
import { CredentialListScreen } from '../screens/CredentialListScreen';

/** The credentials of the project (SPEC-016 R-SH-URL: the project's sections live under its address). */
export const Route = createFileRoute('/_session/orgs/$orgSlug/projects/$projectHandle/credentials')({
  errorComponent: RouteError,
  component: CredentialListScreen,
});
