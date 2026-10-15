// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { zodResolver } from '@hookform/resolvers/zod';
import { Banner, Button, Dialog, FormField, Input, Select } from '@orcon/ui';
import { useId, useState } from 'react';
import { useForm } from 'react-hook-form';
import { useIntl } from 'react-intl';
import { z } from 'zod';

import { CREDENTIAL_TYPES, catalogLocaleOf, credentialText, credentialType, type CredentialType } from '../../../app/credential-store/credential-types';
import { CredentialNameTaken } from '../../../app/credential-store/store';
import { formatIssue } from '../../../app/forms/zod-messages';
import { useCreateCredential } from '../api/mutations';
import { credentialMessages } from './messages';

export interface CreateCredentialDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  projectId: string;
}

interface Values {
  name: string;
  type: string;
  fields: Record<string, string>;
}

/** The fields of the chosen type are the only ones checked; each one is required when the type says so. */
function schemaFor(type: CredentialType | undefined) {
  const fields = Object.fromEntries((type?.fields ?? []).map((field) => [field.name, field.required ? z.string().min(1).max(4096) : z.string().max(4096)]));
  return z.object({ name: z.string().trim().min(1).max(128), type: z.string().min(1), fields: z.object(fields) });
}

const defaultsOf = (type: CredentialType | undefined): Record<string, string> => Object.fromEntries((type?.fields ?? []).map((field) => [field.name, field.default ?? '']));

function CreateForm({ formId, projectId, onClose, onBusy }: { formId: string; projectId: string; onClose: () => void; onBusy: (busy: boolean) => void }) {
  const intl = useIntl();
  const language = catalogLocaleOf(intl.locale);
  const create = useCreateCredential(projectId);
  const [failed, setFailed] = useState(false);
  const first = CREDENTIAL_TYPES[0];
  const form = useForm<Values>({
    resolver: (values, context, options) => zodResolver(schemaFor(credentialType(values.type)), { error: (issue) => formatIssue(intl, issue) })(values, context, options),
    defaultValues: { name: '', type: first?.id ?? '', fields: defaultsOf(first) },
  });
  const typeId = form.watch('type');
  const type = credentialType(typeId);
  const { errors } = form.formState;
  const text = (key: string) => (type ? credentialText(type, language, key) : key);

  const submit = form.handleSubmit(async (values) => {
    setFailed(false);
    onBusy(true);
    try {
      await create.mutateAsync({ name: values.name.trim(), type: values.type, fields: values.fields });
      create.reset();
      onClose();
    } catch (error) {
      create.reset();
      if (error instanceof CredentialNameTaken) form.setError('name', { message: intl.formatMessage(credentialMessages.nameTaken) });
      else setFailed(true);
    } finally {
      onBusy(false);
    }
  });

  return (
    <form id={formId} noValidate onSubmit={(event) => void submit(event)} className="flex flex-col gap-3">
      {failed ? <Banner tone="danger" title={intl.formatMessage(credentialMessages.failed)} /> : null}
      <FormField name="/name" label={intl.formatMessage(credentialMessages.fieldName)} description={intl.formatMessage(credentialMessages.fieldNameHint)} required error={errors.name?.message}>
        <Input autoComplete="off" {...form.register('name')} />
      </FormField>
      <FormField name="/type" label={intl.formatMessage(credentialMessages.fieldType)}>
        <Select
          value={typeId}
          onValueChange={(next) => {
            form.setValue('type', next);
            form.setValue('fields', defaultsOf(credentialType(next)));
          }}
          options={CREDENTIAL_TYPES.map((known) => ({ value: known.id, label: credentialText(known, language, 'name') }))}
        />
      </FormField>
      {(type?.fields ?? []).map((field) => (
        <FormField
          key={`${typeId}-${field.name}`}
          name={`/fields/${field.name}`}
          label={text(`field.${field.name}.label`)}
          description={text(`field.${field.name}.description`)}
          required={field.required}
          error={errors.fields?.[field.name]?.message}
        >
          {/* A secret is write-only: the field starts empty, hides what is typed and is never offered to a password manager as stored. */}
          <Input type={field.secret ? 'password' : 'text'} autoComplete={field.secret ? 'new-password' : 'off'} spellCheck={false} {...form.register(`fields.${field.name}`)} />
        </FormField>
      ))}
    </form>
  );
}

/**
 * Creates a credential in the project. The type decides the fields; secret fields are write-only and the
 * form lives only while the dialog is open, so nothing typed outlives it.
 */
export function CreateCredentialDialog({ open, onOpenChange, projectId }: CreateCredentialDialogProps) {
  const intl = useIntl();
  const formId = useId();
  const [busy, setBusy] = useState(false);
  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={intl.formatMessage(credentialMessages.createTitle)}
      description={intl.formatMessage(credentialMessages.createText)}
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
            {intl.formatMessage(credentialMessages.save)}
          </Button>
        </>
      }
    >
      {open ? (
        <CreateForm
          formId={formId}
          projectId={projectId}
          onBusy={setBusy}
          onClose={() => {
            onOpenChange(false);
          }}
        />
      ) : null}
    </Dialog>
  );
}
