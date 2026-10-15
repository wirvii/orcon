// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { zodResolver } from '@hookform/resolvers/zod';
import { Button, Dialog, FormField, Input, ProblemAlert, type ProblemDetails } from '@orcon/ui';
import { useNavigate } from '@tanstack/react-router';
import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { defineMessages, useIntl } from 'react-intl';
import { z } from 'zod';

import { formatIssue } from '../../../app/forms/zod-messages';
import { toProblemDetails } from '../../../app/errors/to-problem-details';
import { useCreateFlow } from '../api/mutations';

const messages = defineMessages({
  title: { id: 'flows.create.title', defaultMessage: 'New flow', description: 'Title of the dialog that creates a flow.' },
  description: {
    id: 'flows.create.description',
    defaultMessage: 'The flow starts inactive with an empty draft. The editor opens it right away.',
    description: 'Explains what the new flow contains and where it opens.',
  },
  name: { id: 'flows.create.name', defaultMessage: 'Name', description: 'Label of the name field of a new flow.' },
  cancel: { id: 'flows.create.cancel', defaultMessage: 'Cancel', description: 'Button that closes the flow dialog without creating anything.' },
  submit: { id: 'flows.create.submit', defaultMessage: 'Create and open', description: 'Button that creates the flow and opens it in the editor.' },
});

const createFlowSchema = z.object({
  name: z.string().trim().min(1).max(120),
});

export interface CreateFlowDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  orgId: string;
  projectId: string;
  /** The address parts of the project: the editor address carries both (SPEC-016 Q1 = A). */
  orgSlug: string;
  projectHandle: string;
}

/** Creates a flow in the project and opens it in the editor. Failures are shown above the form. */
export function CreateFlowDialog({ open, onOpenChange, orgId, projectId, orgSlug, projectHandle }: CreateFlowDialogProps) {
  const intl = useIntl();
  const navigate = useNavigate();
  const create = useCreateFlow(orgId, projectId);
  const [failure, setFailure] = useState<ProblemDetails | undefined>(undefined);

  const form = useForm<z.infer<typeof createFlowSchema>>({
    resolver: zodResolver(createFlowSchema, { error: (issue) => formatIssue(intl, issue) }),
    defaultValues: { name: '' },
    mode: 'onTouched',
  });
  const { errors } = form.formState;

  const submit = form.handleSubmit(async (values) => {
    setFailure(undefined);
    try {
      const flow = await create.mutateAsync(values.name);
      onOpenChange(false);
      await navigate({ to: '/orgs/$orgSlug/projects/$projectHandle/flows/$flowId', params: { orgSlug, projectHandle, flowId: flow.id } });
    } catch (error) {
      setFailure(toProblemDetails(error));
    }
  });

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={intl.formatMessage(messages.title)}
      description={intl.formatMessage(messages.description)}
      footer={
        <>
          <Button variant="secondary" onClick={() => onOpenChange(false)}>
            {intl.formatMessage(messages.cancel)}
          </Button>
          <Button type="submit" form="create-flow-form" loading={create.isPending}>
            {intl.formatMessage(messages.submit)}
          </Button>
        </>
      }
    >
      <form id="create-flow-form" noValidate onSubmit={(event) => void submit(event)} className="flex flex-col gap-4">
        {failure ? <ProblemAlert problem={failure} /> : null}
        <FormField label={intl.formatMessage(messages.name)} name="/name" required error={errors.name?.message}>
          <Input autoComplete="off" {...form.register('name')} />
        </FormField>
      </form>
    </Dialog>
  );
}
