// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { zodResolver } from '@hookform/resolvers/zod';
import { Banner, Button, Dialog, FormField, Input, ProblemAlert, Skeleton } from '@orcon/ui';
import { useQuery } from '@tanstack/react-query';
import { useId, useState } from 'react';
import { useForm } from 'react-hook-form';
import { useIntl } from 'react-intl';
import { z } from 'zod';

import { problemStatus, toProblemDetails } from '../../../app/errors/to-problem-details';
import { formatIssue } from '../../../app/forms/zod-messages';
import { useReplaceSecret } from '../../../app/credentials/api/mutations';
import { credentialQueryOptions } from '../../../app/credentials/api/queries';
import type { Credential } from '../../../app/credentials/model/credential';

import { changeMessages, credentialMessages } from '../../../app/credentials/model/messages';
import { serviceOfCredential, type Service } from '../../../app/credentials/model/services';

export interface ReplaceSecretDialogProps {
  orgId: string;
  /** The credential whose secret is replaced; the dialog is closed when there is none. */
  credential: Credential | undefined;
  onOpenChange: (open: boolean) => void;
}

/**
 * Replaces the secret values of a credential. Only the secret fields of its service are asked: the other
 * settings stay as they are, and no current value is ever shown (SPEC-004 R-CRED-01..02). The credential is
 * read when the dialog opens and its entity tag travels with the change, so a credential somebody else
 * changed in the meantime is refused instead of overwritten.
 */
export function ReplaceSecretDialog({ orgId, credential, onOpenChange }: ReplaceSecretDialogProps) {
  const intl = useIntl();
  const formId = useId();
  const [busy, setBusy] = useState(false);
  return (
    <Dialog
      open={credential !== undefined}
      onOpenChange={onOpenChange}
      title={intl.formatMessage(changeMessages.rotateTitle, {
        name: credential?.name ?? '',
      })}
      description={intl.formatMessage(changeMessages.rotateText)}
      footer={
        <>
          <Button
            variant="ghost"
            onClick={() => {
              onOpenChange(false);
            }}
          >
            {intl.formatMessage(credentialMessages.cancel)}
          </Button>
          <Button type="submit" form={formId} loading={busy}>
            {intl.formatMessage(changeMessages.rotateSave)}
          </Button>
        </>
      }
    >
      {credential === undefined ? null : (
        <ReplaceLoader
          key={credential.id}
          orgId={orgId}
          credentialId={credential.id}
          formId={formId}
          onBusy={setBusy}
          onDone={() => {
            onOpenChange(false);
          }}
        />
      )}
    </Dialog>
  );
}

function ReplaceLoader({ orgId, credentialId, formId, onBusy, onDone }: { orgId: string; credentialId: string; formId: string; onBusy: (busy: boolean) => void; onDone: () => void }) {
  const intl = useIntl();
  const loaded = useQuery(credentialQueryOptions(orgId, credentialId));
  if (loaded.status === 'pending') return <Skeleton role="status" aria-label={intl.formatMessage(changeMessages.loading)} className="h-16 w-full" />;
  if (loaded.status === 'error') return <Banner tone="danger" title={intl.formatMessage(changeMessages.loadFailed)} />;
  const service = serviceOfCredential(loaded.data.credential);
  if (service === undefined) return <Banner tone="warning" title={intl.formatMessage(changeMessages.unknownService)} />;
  return <ReplaceForm formId={formId} credentialId={credentialId} etag={loaded.data.etag} service={service} publicFields={loaded.data.credential.publicFields} onBusy={onBusy} onDone={onDone} />;
}

interface ReplaceFormProps {
  formId: string;
  credentialId: string;
  etag: string;
  service: Service;
  publicFields: Readonly<Record<string, string>> | undefined;
  onBusy: (busy: boolean) => void;
  onDone: () => void;
}

function ReplaceForm({ formId, credentialId, etag, service, publicFields, onBusy, onDone }: ReplaceFormProps) {
  const intl = useIntl();
  const replace = useReplaceSecret();
  const [failure, setFailure] = useState<unknown>(undefined);
  const secrets = service.fields.filter((field) => field.secret);
  const schema = z.object(Object.fromEntries(secrets.map((field) => [field.name, field.required ? z.string().min(1).max(65536) : z.string().max(65536)])));
  const form = useForm<Record<string, string>>({
    resolver: zodResolver(schema, {
      error: (issue) => formatIssue(intl, issue),
    }),
    defaultValues: Object.fromEntries(secrets.map((field) => [field.name, ''])),
  });
  const { errors } = form.formState;

  const submit = form.handleSubmit(async (values) => {
    setFailure(undefined);
    onBusy(true);
    try {
      await replace.mutateAsync({
        credentialId,
        etag,
        // The new version holds only what is typed: an optional value left empty is not stored.
        secretFields: Object.fromEntries(Object.entries(values).filter(([, value]) => value !== '')),
        publicFields,
      });
      replace.reset();
      onDone();
    } catch (error) {
      replace.reset();
      setFailure(error);
    } finally {
      onBusy(false);
    }
  });

  return (
    <form id={formId} noValidate onSubmit={(event) => void submit(event)} className="flex flex-col gap-3">
      {failure === undefined ? null : problemStatus(failure) === 412 ? (
        <Banner tone="warning" title={intl.formatMessage(changeMessages.stale)} />
      ) : (
        <ProblemAlert problem={toProblemDetails(failure)} />
      )}
      <p className="text-fg-muted text-sm">{intl.formatMessage(changeMessages.currentValueHidden)}</p>
      {secrets.length > 1 ? <p className="text-fg-muted text-sm">{intl.formatMessage(changeMessages.replacesAll)}</p> : null}
      {secrets.map((field) => (
        <FormField
          key={field.name}
          name={`/secretFields/${field.name}`}
          label={intl.formatMessage(field.label)}
          description={intl.formatMessage(field.hint)}
          required={field.required}
          error={errors[field.name]?.message}
        >
          <Input type="password" autoComplete="new-password" spellCheck={false} {...form.register(field.name)} />
        </FormField>
      ))}
    </form>
  );
}
