// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { zodResolver } from '@hookform/resolvers/zod';
import { Banner, Button, Dialog, FormField, Input, Select } from '@orcon/ui';
import { useId, useState } from 'react';
import { useForm } from 'react-hook-form';
import { useIntl } from 'react-intl';
import { z } from 'zod';

import { formatIssue } from '../../../../app/forms/zod-messages';
import { CredentialNameTaken, type CredentialSummary } from '../../api/credential-source';
import { useCreateCredential } from '../../api/mutations';
import { catalogLocaleOf, credentialText, credentialType, type CredentialType } from '../../../../app/credential-store/credential-types';

import { configMessages } from './messages';

export interface CreateCredentialDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** The credential types that fit the slot. */
  types: readonly string[];
  /** A name to start with, when the dialog answers a missing credential. */
  initialName?: string | undefined;
  onCreated: (credential: CredentialSummary) => void;
}

interface Values {
  name: string;
  type: string;
  fields: Record<string, string>;
}

function schemaFor(type: CredentialType | undefined) {
  const fields = Object.fromEntries((type?.fields ?? []).map((field) => [field.name, field.required ? z.string().min(1).max(4096) : z.string().max(4096)]));
  return z.object({ name: z.string().trim().min(1).max(128), type: z.string().min(1), fields: z.object(fields) });
}

function CredentialForm({ formId, types, initialName, onCreated, onClose, onBusy }: { formId: string } & Pick<CreateCredentialDialogProps, 'types' | 'initialName' | 'onCreated'> & { onClose: () => void; onBusy: (busy: boolean) => void }) {
  const intl = useIntl();
  const language = catalogLocaleOf(intl.locale);
  const create = useCreateCredential();
  const [failed, setFailed] = useState(false);
  const defaultType = types[0] ?? '';
  const form = useForm<Values>({
    // The schema follows the type chosen: its fields are the ones the type declares.
    resolver: (values, context, options) => zodResolver(schemaFor(credentialType(values.type)), { error: (issue) => formatIssue(intl, issue) })(values, context, options),
    defaultValues: {
      name: initialName ?? '',
      type: defaultType,
      fields: Object.fromEntries((credentialType(defaultType)?.fields ?? []).map((field) => [field.name, field.default ?? ''])),
    },
  });
  const typeId = form.watch('type');
  const type = credentialType(typeId);
  const { errors } = form.formState;
  const text = (key: string) => (type ? credentialText(type, language, key) : key);
  const typeName = text('name');

  const submit = form.handleSubmit(
    async (values) => {
      setFailed(false);
      onBusy(true);
      try {
        const created = await create.mutateAsync({ name: values.name.trim(), type: values.type, fields: values.fields });
        create.reset();
        onCreated(created);
        onClose();
      } catch (error) {
        create.reset();
        if (error instanceof CredentialNameTaken) form.setError('name', { message: intl.formatMessage(configMessages.credentialNameTaken) });
        else setFailed(true);
      } finally {
        onBusy(false);
      }
    },
  );

  return (
    <form id={formId} noValidate onSubmit={(event) => void submit(event)} className="flex flex-col gap-3">
      {failed ? <Banner tone="danger" title={intl.formatMessage(configMessages.credentialFailed)} /> : null}
      <FormField name="/name" label={intl.formatMessage(configMessages.credentialName)} description={intl.formatMessage(configMessages.credentialNameHint)} required error={errors.name?.message}>
        <Input autoComplete="off" {...form.register('name')} />
      </FormField>
      {types.length > 1 ? (
        <FormField name="/type" label={intl.formatMessage(configMessages.credentialType)}>
          <Select
            value={typeId}
            onValueChange={(next) => {
              form.setValue('type', next);
              form.setValue('fields', Object.fromEntries((credentialType(next)?.fields ?? []).map((field) => [field.name, field.default ?? ''])));
            }}
            options={types.map((id) => {
              const known = credentialType(id);
              return { value: id, label: known ? credentialText(known, language, 'name') : id };
            })}
          />
        </FormField>
      ) : type ? (
        <p className="text-fg-muted text-sm">
          <span className="font-medium">{intl.formatMessage(configMessages.credentialType)}: </span>
          {typeName}
        </p>
      ) : null}
      {(type?.fields ?? []).map((field) => (
        <FormField
          key={field.name}
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
 * The inline dialog that creates a credential from a node (SPEC-014 R-ED-CRED-02): the fields come
 * from the credential type, secret fields are write-only, and the new credential is chosen for the
 * slot when it is created. The form lives only while the dialog is open, so nothing typed outlives it.
 */
export function CreateCredentialDialog({ open, onOpenChange, types, initialName, onCreated }: CreateCredentialDialogProps) {
  const intl = useIntl();
  const formId = useId();
  const [busy, setBusy] = useState(false);
  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={intl.formatMessage(configMessages.credentialDialogTitle)}
      description={intl.formatMessage(configMessages.credentialDialogText)}
      footer={
        <>
          <Button
            variant="ghost"
            onClick={() => {
              onOpenChange(false);
            }}
          >
            {intl.formatMessage(configMessages.credentialCancel)}
          </Button>
          <Button type="submit" form={formId} loading={busy}>
            {intl.formatMessage(configMessages.credentialSave)}
          </Button>
        </>
      }
    >
      {open ? (
        <CredentialForm
          formId={formId}
          types={types}
          initialName={initialName}
          onCreated={onCreated}
          onClose={() => {
            onOpenChange(false);
          }}
          onBusy={setBusy}
        />
      ) : null}
    </Dialog>
  );
}
