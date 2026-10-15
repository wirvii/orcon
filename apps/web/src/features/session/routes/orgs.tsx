// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createFileRoute } from '@tanstack/react-router';

import { RouteError } from '../../../app/errors/RouteError';
import { OrganizationPickerScreen } from '../screens/OrganizationPickerScreen';

/** The organization picker: the organizations of the person, each one a link to its projects. */
export const Route = createFileRoute('/_session/orgs')({
  errorComponent: RouteError,
  component: OrganizationPickerScreen,
});
