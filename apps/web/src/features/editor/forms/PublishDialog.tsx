// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { zodResolver } from '@hookform/resolvers/zod';
import { Button, Dialog, FormField, ProblemAlert, Textarea, type ProblemDetails } from '@orcon/ui';
import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { defineMessages, useIntl } from 'react-intl';
import { z } from 'zod';

import { formatIssue } from '../../../app/forms/zod-messages';
import { toProblemDetails } from '../../../app/errors/to-problem-details';
import { usePublishFlow } from '../api/mutations';

/** The longest release note the server stores (SPEC-005 `PublishFlowRequest`). */
const MAX_NOTE = 500;

const messages = defineMessages({
  title: {
    id: 'editor.publish.title',
    defaultMessage: 'Publish this version',
    description: 'Title of the dialog that publishes the draft of the flow as a new version.',
  },
  description: {
    id: 'editor.publish.description',
    defaultMessage: 'New runs use the published version while the flow is active. Runs already in progress keep the version they started with.',
    description: 'Explains what publishing changes for runs that start after it and for runs in progress.',
  },
  note: { id: 'editor.publish.note', defaultMessage: 'Release note (optional)', description: 'Label of the optional note stored with the published version.' },
  noteHelp: {
    id: 'editor.publish.noteHelp',
    defaultMessage: 'Say what changed, so the next person understands this version. Up to 500 characters.',
    description: 'Helper text under the release note field: what to write and the limit.',
  },
  cancel: { id: 'editor.publish.cancel', defaultMessage: 'Cancel', description: 'Button that closes the publish dialog without publishing.' },
  submit: { id: 'editor.publish.submit', defaultMessage: 'Publish', description: 'Button that publishes the draft as a new version.' },
  tooLong: {
    id: 'editor.publish.tooLong',
    defaultMessage: 'The release note is longer than 500 characters.',
    description: 'Error under the release note when it is longer than the limit.',
  },
});

const publishSchema = z.object({
  note: z.string().max(MAX_NOTE, { message: 'too-long' }),
});

type PublishValues = z.infer<typeof publishSchema>;

export interface PublishDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  flowId: string;
}

/**
 * Publishes the draft as a new version with an optional release note. A draft the server refuses (for
 * example one with errors) is shown above the form, and the dialog stays open so the person can fix it.
 */
export function PublishDialog({ open, onOpenChange, flowId }: PublishDialogProps) {
  const intl = useIntl();
  const publish = usePublishFlow(flowId);
  const [failure, setFailure] = useState<ProblemDetails | undefined>(undefined);

  const form = useForm<PublishValues>({
    resolver: zodResolver(publishSchema, {
      error: (issue) => (issue.code === 'too_big' ? intl.formatMessage(messages.tooLong) : formatIssue(intl, issue)),
    }),
    defaultValues: { note: '' },
    mode: 'onTouched',
  });
  const { errors } = form.formState;

  const submit = form.handleSubmit(async (values) => {
    setFailure(undefined);
    try {
      const note = values.note.trim();
      await publish.mutateAsync(note === '' ? undefined : note);
      form.reset({ note: '' });
      onOpenChange(false);
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
          <Button type="submit" form="publish-flow-form" loading={publish.isPending}>
            {intl.formatMessage(messages.submit)}
          </Button>
        </>
      }
    >
      <form id="publish-flow-form" noValidate onSubmit={(event) => void submit(event)} className="flex flex-col gap-4">
        {failure ? <ProblemAlert problem={failure} /> : null}
        <FormField
          label={intl.formatMessage(messages.note)}
          name="/note"
          description={intl.formatMessage(messages.noteHelp)}
          error={errors.note?.message}
        >
          <Textarea rows={4} {...form.register('note')} />
        </FormField>
      </form>
    </Dialog>
  );
}
