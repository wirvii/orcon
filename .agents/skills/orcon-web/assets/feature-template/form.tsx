// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0
//
// Template for features/<area>/forms/<name>-form.tsx (forms.md FM-1 to FM-9,
// errors.md ER-1 to ER-3). The schema is the generated request schema; field
// names are JSON pointers so server errors land on the right field. Layout,
// components and strings follow orcon-ui.

import { zodResolver } from '@hookform/resolvers/zod';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useForm } from 'react-hook-form';
import { FormattedMessage } from 'react-intl';
import { updateWidgetRequestSchema } from '@orcon/api-contract/zod';
import type { OrgId, UpdateWidgetRequest, Widget } from '@orcon/sdk';
import { Button, ErrorSummary, FormField, Input, ProblemErrorsProvider } from '@orcon/ui';
import { toProblemDetails } from '../../../app/errors/to-problem-details';
import { zodIssueMessage } from '../../../app/forms/zod-messages';
import { widgetMutations } from '../api/mutations';

type WidgetFormProps = {
  readonly orgId: OrgId;
  readonly widget: Widget;
};

export function WidgetForm({ orgId, widget }: WidgetFormProps) {
  const queryClient = useQueryClient();
  const update = useMutation(widgetMutations.update(orgId, queryClient));
  const form = useForm<UpdateWidgetRequest>({
    resolver: zodResolver(updateWidgetRequestSchema),
    mode: 'onTouched',
    reValidateMode: 'onChange',
    // Server data enters through values; dirty fields survive a refetch.
    values: { name: widget.name },
    resetOptions: { keepDirtyValues: true },
  });

  const problem = update.error ? toProblemDetails(update.error) : undefined;

  const onSubmit = form.handleSubmit((body) => {
    update.mutate({ id: widget.id, revision: widget.revision, body });
  });

  return (
    <ProblemErrorsProvider problem={problem} setError={form.setError}>
      <form noValidate onSubmit={(event) => void onSubmit(event)}>
        <ErrorSummary />
        <FormField
          name="/name"
          label={<FormattedMessage id="widgets.form.name" />}
          error={zodIssueMessage(form.formState.errors.name)}
        >
          <Input {...form.register('name')} />
        </FormField>
        <Button type="submit" disabled={update.isPending}>
          <FormattedMessage id="widgets.form.save" />
        </Button>
      </form>
    </ProblemErrorsProvider>
  );
}
