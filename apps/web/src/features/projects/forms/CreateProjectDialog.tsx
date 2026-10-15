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
import { problemStatus, toProblemDetails } from '../../../app/errors/to-problem-details';
import { useCreateProject } from '../api/mutations';

const messages = defineMessages({
  title: { id: 'projects.create.title', defaultMessage: 'New project', description: 'Title of the dialog that creates a project.' },
  description: {
    id: 'projects.create.description',
    defaultMessage: 'You become its administrator, so you can see it and add people later.',
    description: 'Explains that the person who creates a project can see it.',
  },
  name: { id: 'projects.create.name', defaultMessage: 'Name', description: 'Label of the name field of a new project.' },
  environment: { id: 'projects.create.environment', defaultMessage: 'Environment', description: 'Label of the optional environment field of a new project.' },
  environmentHelp: {
    id: 'projects.create.environmentHelp',
    defaultMessage: 'Optional. A short label such as production or staging.',
    description: 'Helper text under the optional environment field.',
  },
  cancel: { id: 'projects.create.cancel', defaultMessage: 'Cancel', description: 'Button that closes the project dialog without creating anything.' },
  submit: { id: 'projects.create.submit', defaultMessage: 'Create project', description: 'Button that creates the project.' },
  conflict: {
    id: 'projects.create.conflict',
    defaultMessage: 'This name is not available. Choose another name.',
    description: 'Shown under the name when the project cannot take it. It never says where the name is used.',
  },
});

const createProjectSchema = z.object({
  name: z.string().trim().min(1).max(120),
  environment: z.string().trim().max(64),
});

type CreateProjectValues = z.infer<typeof createProjectSchema>;

export interface CreateProjectDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  orgId: string;
  orgSlug: string;
}

/**
 * Creates a project and opens its flows. The server errors on the fields are shown under the field
 * they name; a name already taken gets a generic message, and any other failure is shown above the form.
 */
export function CreateProjectDialog({ open, onOpenChange, orgId, orgSlug }: CreateProjectDialogProps) {
  const intl = useIntl();
  const navigate = useNavigate();
  const create = useCreateProject(orgId);
  const [failure, setFailure] = useState<ProblemDetails | undefined>(undefined);

  const form = useForm<CreateProjectValues>({
    resolver: zodResolver(createProjectSchema, { error: (issue) => formatIssue(intl, issue) }),
    defaultValues: { name: '', environment: '' },
    mode: 'onTouched',
  });
  const { errors } = form.formState;

  const submit = form.handleSubmit(async (values) => {
    setFailure(undefined);
    try {
      const project = await create.mutateAsync({ name: values.name, environment: values.environment === '' ? undefined : values.environment });
      onOpenChange(false);
      await navigate({ to: '/orgs/$orgSlug/projects/$projectHandle/flows', params: { orgSlug, projectHandle: project.handle } });
    } catch (error) {
      if (problemStatus(error) === 409) {
        form.setError('name', { message: intl.formatMessage(messages.conflict) });
        return;
      }
      const problem = toProblemDetails(error);
      let mapped = false;
      for (const fieldError of problem.errors ?? []) {
        if (fieldError.pointer === '/name' || fieldError.pointer === '/environment') {
          form.setError(fieldError.pointer === '/name' ? 'name' : 'environment', { message: fieldError.detail });
          mapped = true;
        }
      }
      if (!mapped) setFailure(problem);
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
          <Button type="submit" form="create-project-form" loading={create.isPending}>
            {intl.formatMessage(messages.submit)}
          </Button>
        </>
      }
    >
      <form id="create-project-form" noValidate onSubmit={(event) => void submit(event)} className="flex flex-col gap-4">
        {failure ? <ProblemAlert problem={failure} /> : null}
        <FormField label={intl.formatMessage(messages.name)} name="/name" required error={errors.name?.message}>
          <Input autoComplete="off" {...form.register('name')} />
        </FormField>
        <FormField
          label={intl.formatMessage(messages.environment)}
          name="/environment"
          description={intl.formatMessage(messages.environmentHelp)}
          error={errors.environment?.message}
        >
          <Input autoComplete="off" {...form.register('environment')} />
        </FormField>
      </form>
    </Dialog>
  );
}
