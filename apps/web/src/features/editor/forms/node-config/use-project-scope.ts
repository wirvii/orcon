// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { useRouteContext } from '@tanstack/react-router';

export interface ProjectScope {
  readonly orgId: string;
  readonly projectId: string;
  readonly projectName: string;
}

interface ScopedContext {
  readonly organization?: { readonly id: string };
  readonly project?: { readonly id: string; readonly name: string };
}

/**
 * The organization and project of the address the editor is open under, which the credential list and the
 * credentials a step creates belong to. `undefined` outside a project address, where there is nothing to list.
 */
export function useProjectScope(): ProjectScope | undefined {
  const context: ScopedContext = useRouteContext({ strict: false });
  const { organization, project } = context;
  return organization === undefined || project === undefined ? undefined : { orgId: organization.id, projectId: project.id, projectName: project.name };
}
