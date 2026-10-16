// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { zodResolver } from '@hookform/resolvers/zod';
import { Banner, Button, FormField, Input, ProblemAlert } from '@orcon/ui';
import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { defineMessages, useIntl } from 'react-intl';
import { z } from 'zod';

import { problemStatus, toProblemDetails } from '../../../app/errors/to-problem-details';
import { formatIssue } from '../../../app/forms/zod-messages';
import { hasDisplayName } from '../../../app/session/identity';
import { useRenameUser } from '../api/mutations';
import type { UserWithTag } from '../api/operations';

const messages = defineMessages({
  label: { id: 'settings.profile.form.name', defaultMessage: 'Name', description: 'Label of the field where the person types their name on the profile card.' },
  hint: {
    id: 'settings.profile.form.hint',
    defaultMessage: 'Shown in the menu and in the greeting. Up to 200 characters.',
    description: 'Hint under the name field of the profile card.',
  },
  save: { id: 'settings.profile.form.save', defaultMessage: 'Save name', description: 'Button that saves the new name of the person.' },
  saved: { id: 'settings.profile.form.saved', defaultMessage: 'Name saved.', description: 'Confirmation, read aloud, after the name was saved.' },
  stale: {
    id: 'settings.profile.form.stale',
    defaultMessage: 'Someone else changed this account while you were editing, so nothing was saved. Load the current name and try again.',
    description: 'Warning shown when the server refuses a name change because the account changed in the meantime.',
  },
  reload: { id: 'settings.profile.form.reload', defaultMessage: 'Load the current name', description: 'Button that reads the account again after a refused change.' },
});

export interface DisplayNameFormProps {
  /** The person and the entity tag the change is based on. */
  profile: UserWithTag;
  /** Reads the person again and returns them: used after the server refused a change because somebody else made one. */
  onReload: () => Promise<UserWithTag | undefined>;
}

/**
 * The name of the signed-in person. The tag read with the person travels in `If-Match`, so a change made by someone
 * else in the meantime is refused (412) and explained instead of overwritten. An account without a name of its own
 * ("Owner" counts as unset) starts with an empty field.
 */
export function DisplayNameForm({ profile, onReload }: DisplayNameFormProps) {
  const intl = useIntl();
  const rename = useRenameUser();
  const [failure, setFailure] = useState<unknown>(undefined);
  const [saved, setSaved] = useState(false);
  const initial = hasDisplayName(profile.user) ? profile.user.displayName : '';

  const schema = z.object({ displayName: z.string().trim().min(1).max(200) });
  const form = useForm<z.infer<typeof schema>>({
    resolver: zodResolver(schema, { error: (issue) => formatIssue(intl, issue) }),
    defaultValues: { displayName: initial },
  });
  const { errors } = form.formState;

  const submit = form.handleSubmit(async ({ displayName }) => {
    setFailure(undefined);
    setSaved(false);
    try {
      await rename.mutateAsync({ displayName, etag: profile.etag });
      form.reset({ displayName });
      setSaved(true);
    } catch (error) {
      for (const entry of toProblemDetails(error).errors ?? []) {
        if (entry.pointer === '/displayName') form.setError('displayName', { message: entry.detail });
      }
      setFailure(error);
    }
  });

  const status = failure === undefined ? undefined : problemStatus(failure);
  return (
    <form noValidate onSubmit={(event) => void submit(event)} className="flex flex-col gap-3">
      {status === 412 ? (
        <Banner
          tone="warning"
          title={intl.formatMessage(messages.stale)}
          action={
            <Button
              variant="secondary"
              onClick={() => {
                setFailure(undefined);
                void onReload().then((fresh) => {
                  if (fresh !== undefined) form.reset({ displayName: hasDisplayName(fresh.user) ? fresh.user.displayName : '' });
                });
              }}
            >
              {intl.formatMessage(messages.reload)}
            </Button>
          }
        />
      ) : failure === undefined ? null : (
        <ProblemAlert problem={toProblemDetails(failure)} />
      )}
      <FormField name="/displayName" label={intl.formatMessage(messages.label)} description={intl.formatMessage(messages.hint)} required error={errors.displayName?.message}>
        <Input autoComplete="name" {...form.register('displayName')} />
      </FormField>
      <div className="flex items-center gap-3">
        <Button type="submit" loading={rename.isPending}>
          {intl.formatMessage(messages.save)}
        </Button>
        <span role="status" aria-live="polite" className="text-fg-muted text-sm">
          {saved ? intl.formatMessage(messages.saved) : ''}
        </span>
      </div>
    </form>
  );
}
