// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { useMutation, useQueryClient } from '@tanstack/react-query';

import { createFlow } from './operations';
import { flowKeys } from './queries';

/**
 * Creates a flow in the project. It is never retried; on success every list of flows and the flow counts are stale.
 * The new flow is read again by the editor, so nothing else is written here.
 */
export function useCreateFlow(orgId: string, projectId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationKey: [...flowKeys.all(orgId), 'create', projectId],
    mutationFn: (name: string) => createFlow({ projectId, name }),
    retry: false,
    onSuccess: async () => {
      // Every list of flows of the organization (the project's, the home page's) and the flow counts of the projects.
      await Promise.all([queryClient.invalidateQueries({ queryKey: flowKeys.all(orgId) }), queryClient.invalidateQueries({ queryKey: [orgId, 'projects'] })]);
    },
  });
}
