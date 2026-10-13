// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0
//
// Template for features/<area>/api/mutations.ts (data-and-live-updates.md
// DT-6, DT-17). Editable resources are sent with If-Match; a 412 response
// opens the conflict flow. Idempotency keys are added by the SDK.

import { mutationOptions, type QueryClient } from '@tanstack/react-query';
import { widgetSchema } from '@orcon/api-contract/zod';
import type { OrgId, UpdateWidgetRequest, WidgetId } from '@orcon/sdk';
import { sdk } from '@orcon/sdk';
import { parseWith } from '../../../app/boundary/parse';
import { widgetKeys } from './queries';

type UpdateWidgetInput = {
  readonly id: WidgetId;
  readonly revision: string;
  readonly body: UpdateWidgetRequest;
};

export const widgetMutations = {
  update: (org: OrgId, queryClient: QueryClient) =>
    mutationOptions({
      mutationKey: [...widgetKeys.all(org), 'update'],
      mutationFn: async ({ id, revision, body }: UpdateWidgetInput) =>
        parseWith(widgetSchema, await sdk.widgets.update({ org, id, ifMatch: revision, body })),
      // Write the returned resource and invalidate only the lists that show it.
      onSuccess: async (widget) => {
        queryClient.setQueryData(widgetKeys.detail(org, widget.id), widget);
        await queryClient.invalidateQueries({ queryKey: widgetKeys.lists(org) });
      },
    }),
};
