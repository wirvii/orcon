// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { zodResolver } from '@hookform/resolvers/zod';
import { Banner, Button, Dialog, FormField, Input } from '@orcon/ui';
import { useId, useState } from 'react';
import { useForm } from 'react-hook-form';
import { useIntl } from 'react-intl';
import { z } from 'zod';

import { catalogLocaleOf, credentialText, credentialType } from '../../../app/credential-store/credential-types';
import type { CredentialSummary } from '../../../app/credential-store/store';
import { formatIssue } from '../../../app/forms/zod-messages';
import { useRotateCredential } from '../api/mutations';
import { credentialMessages } from './messages';

export interface RotateCredentialDialogProps {
  /** The credential whose secret is replaced; the dialog is closed when there is none. */
  credential: CredentialSummary | undefined;
  onOpenChange: (open: boolean) => void;
}

/**
 * Replaces the secret values of a credential. Only the secret fields of its type are asked: the other
 * settings stay as they are, and no current value is ever shown (SPEC-004 R-CRED-01..02).
 */
export function RotateCredentialDialog({ credential, onOpenChange }: RotateCredentialDialogProps) {
  const intl = useIntl();
  const formId = useId();
  const [busy, setBusy] = useState(false);
  const open = credential !== undefined;
  const type = credential === undefined ? undefined : credentialType(credential.type);
  const language = catalogLocaleOf(intl.locale);
  const secrets = (type?.fields ?? []).filter((field) => field.secret);
  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={intl.formatMessage(credentialMessages.rotateTitle, { name: credential?.name ?? '' })}
      description={intl.formatMessage(credentialMessages.rotateText)}
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
            {intl.formatMessage(credentialMessages.rotateSave)}
          </Button>
        </>
      }
    >
      {credential !== undefined && type !== undefined ? (
        <RotateForm
          key={credential.id}
          formId={formId}
          credentialId={credential.id}
          fields={secrets.map((field) => field.name)}
          labels={Object.fromEntries(secrets.map((field) => [field.name, { label: credentialText(type, language, `field.${field.name}.label`), description: credentialText(type, language, `field.${field.name}.description`) }]))}
          onBusy={setBusy}
          onDone={() => {
            onOpenChange(false);
          }}
        />
      ) : null}
    </Dialog>
  );
}

interface RotateFormProps {
  formId: string;
  credentialId: string;
  fields: readonly string[];
  labels: Readonly<Record<string, { label: string; description: string }>>;
  onBusy: (busy: boolean) => void;
  onDone: () => void;
}

function RotateForm({ formId, credentialId, fields, labels, onBusy, onDone }: RotateFormProps) {
  const intl = useIntl();
  const rotate = useRotateCredential();
  const [failed, setFailed] = useState(false);
  const schema = z.object(Object.fromEntries(fields.map((name) => [name, z.string().min(1).max(4096)])));
  const form = useForm<Record<string, string>>({
    resolver: zodResolver(schema, { error: (issue) => formatIssue(intl, issue) }),
    defaultValues: Object.fromEntries(fields.map((name) => [name, ''])),
  });
  const { errors } = form.formState;

  const submit = form.handleSubmit(async (values) => {
    setFailed(false);
    onBusy(true);
    try {
      await rotate.mutateAsync({ credentialId, fields: values });
      rotate.reset();
      onDone();
    } catch {
      rotate.reset();
      setFailed(true);
    } finally {
      onBusy(false);
    }
  });

  return (
    <form id={formId} noValidate onSubmit={(event) => void submit(event)} className="flex flex-col gap-3">
      {failed ? <Banner tone="danger" title={intl.formatMessage(credentialMessages.failed)} /> : null}
      {fields.map((name) => (
        <FormField key={name} name={`/${name}`} label={labels[name]?.label ?? name} description={labels[name]?.description} required error={errors[name]?.message}>
          <Input type="password" autoComplete="new-password" spellCheck={false} {...form.register(name)} />
        </FormField>
      ))}
    </form>
  );
}
