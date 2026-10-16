// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createFileRoute } from '@tanstack/react-router';

import { RouteError } from '../../../app/errors/RouteError';
import { ProjectSettingsScreen } from '../screens/ProjectSettingsScreen';

/** The settings tab of a project: what it is called and how it is protected. */
export const Route = createFileRoute('/_session/orgs/$orgSlug/projects/$projectHandle/settings')({
  errorComponent: RouteError,
  component: ProjectSettingsScreen,
});
