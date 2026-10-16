// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createFileRoute } from '@tanstack/react-router';

import { PickAnExecution } from '../screens/ExecutionsScreen';

/** The right side of the list while no run is open. */
export const Route = createFileRoute('/_session/orgs/$orgSlug/executions/')({
  component: PickAnExecution,
});
