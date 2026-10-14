// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { useMutation, useQueryClient } from '@tanstack/react-query';

import { createProject, type NewProject } from './operations';
import { projectKeys } from './queries';

/**
 * Creates a project in the organization. It is never retried, and on success every project read of
 * the organization is stale: the list and the handles.
 */
export function useCreateProject(orgId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationKey: [...projectKeys.all(orgId), 'create'],
    mutationFn: (input: Omit<NewProject, 'orgId'>) => createProject({ orgId, ...input }),
    retry: false,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: projectKeys.all(orgId) }),
  });
}
