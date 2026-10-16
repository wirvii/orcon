// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { zodResolver } from '@hookform/resolvers/zod';
import { Button, Dialog, FormField, Input, ProblemAlert } from '@orcon/ui';
import { useQuery } from '@tanstack/react-query';
import { Lock } from 'lucide-react';
import { useId, useState } from 'react';
import { useForm } from 'react-hook-form';
import { useIntl, type IntlShape } from 'react-intl';
import { z } from 'zod';

import { formatIssue } from '../../../app/forms/zod-messages';
import { problemStatus, toProblemDetails } from '../../../app/errors/to-problem-details';
import { useCreateCredential } from '../api/mutations';
import { serviceIconsQueryOptions } from '../api/queries';
import { AllowedHostsField } from '../components/AllowedHostsField';
import { ConnectionGuide } from '../components/ConnectionGuide';
import { ServiceIcon } from '../components/ServiceIcon';
import { ServiceList } from '../components/ServiceList';

import { MAX_ALLOWED_HOSTS } from '../model/hosts';
import { createMessages, credentialMessages, hostMessages } from '../model/messages';
import { SERVICES, defaultFieldValues, serviceById, type Service } from '../model/services';

export interface CreateCredentialDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  orgId: string;
  projectId: string;
  /** The project's name, said in the text so nobody wonders where the credential goes. */
  projectName: string;
  /**
   * Checks a credential before it is saved. The contract has no such operation yet, so the screen passes none and
   * the button is not drawn: a button that cannot work is worse than none.
   */
  onTestConnection?: (() => void) | undefined;
}

interface Values {
  name: string;
  service: string;
  fields: Record<string, string>;
  allowedHosts: string[];
}

/** The fields of the chosen service are the only ones checked; a field is required when the service says so. */
function schemaFor(service: Service | undefined, intl: IntlShape) {
  const fields = Object.fromEntries((service?.fields ?? []).map((field) => [field.name, field.required ? z.string().trim().min(1).max(4096) : z.string().max(4096)]));
  return z.object({
    name: z.string().trim().min(1).max(100),
    service: z.string().min(1),
    fields: z.object(fields),
    allowedHosts: z.array(z.string()).min(1, intl.formatMessage(hostMessages.required)).max(MAX_ALLOWED_HOSTS),
  });
}

function CreateForm({
  formId,
  orgId,
  projectId,
  service,
  onService,
  onClose,
  onBusy,
}: {
  formId: string;
  orgId: string;
  projectId: string;
  service: Service;
  onService: (id: string) => void;
  onClose: () => void;
  onBusy: (busy: boolean) => void;
}) {
  const intl = useIntl();
  const create = useCreateCredential(projectId);
  const icons = useQuery(serviceIconsQueryOptions(orgId, projectId));
  const [failure, setFailure] = useState<unknown>(undefined);
  const form = useForm<Values>({
    resolver: (values, context, options) =>
      zodResolver(schemaFor(serviceById(values.service), intl), {
        error: (issue) => formatIssue(intl, issue),
      })(values, context, options),
    defaultValues: {
      name: '',
      service: service.id,
      fields: defaultFieldValues(service),
      allowedHosts: [],
    },
  });
  const { errors } = form.formState;
  const hosts = form.watch('allowedHosts');

  const choose = (id: string) => {
    const next = serviceById(id);
    if (next === undefined) return;
    onService(id);
    form.setValue('service', id);
    form.setValue('fields', defaultFieldValues(next));
    form.clearErrors('fields');
  };

  const submit = form.handleSubmit(async (values) => {
    setFailure(undefined);
    onBusy(true);
    try {
      const chosen = serviceById(values.service) ?? service;
      const secretFields: Record<string, string> = {};
      const publicFields: Record<string, string> = {};
      for (const field of chosen.fields) {
        const value = (values.fields[field.name] ?? '').trim();
        if (field.secret) secretFields[field.name] = values.fields[field.name] ?? '';
        else if (value !== '') publicFields[field.name] = value;
      }
      await create.mutateAsync({
        name: values.name.trim(),
        type: chosen.connector,
        allowedHosts: values.allowedHosts,
        publicFields,
        secretFields,
      });
      create.reset();
      onClose();
    } catch (error) {
      create.reset();
      if (problemStatus(error) === 409)
        form.setError('name', {
          message: intl.formatMessage(createMessages.nameTaken),
        });
      else setFailure(error);
    } finally {
      onBusy(false);
    }
  });

  return (
    <form id={formId} noValidate onSubmit={(event) => void submit(event)} className="grid gap-6 lg:grid-cols-4">
      <div className="lg:col-span-1">
        <ServiceList services={SERVICES} selectedId={service.id} onSelect={choose} icons={icons.data} />
      </div>
      <section aria-label={intl.formatMessage(createMessages.formHeading)} className="flex flex-col gap-3 lg:col-span-2">
        <div className="flex items-center gap-3">
          <span aria-hidden className="shrink-0">
            <ServiceIcon connector={service.connector} name={intl.formatMessage(service.name)} icons={icons.data} />
          </span>
          <div className="flex min-w-0 flex-col">
            <h3 className="font-semibold wrap-anywhere">{intl.formatMessage(service.name)}</h3>
            <p className="text-fg-muted text-sm">{intl.formatMessage(service.summary)}</p>
          </div>
        </div>
        {failure === undefined ? null : <ProblemAlert problem={toProblemDetails(failure)} />}
        <FormField name="/name" label={intl.formatMessage(createMessages.fieldName)} description={intl.formatMessage(createMessages.fieldNameHint)} required error={errors.name?.message}>
          <Input autoComplete="off" {...form.register('name')} />
        </FormField>
        {service.fields.map((field) => (
          <FormField
            key={`${service.id}-${field.name}`}
            name={`/${field.secret ? 'secretFields' : 'publicFields'}/${field.name}`}
            label={intl.formatMessage(field.label)}
            description={intl.formatMessage(field.hint)}
            required={field.required}
            error={errors.fields?.[field.name]?.message}
            labelAction={
              field.secret ? (
                <span className="text-fg-muted flex items-center gap-1 text-xs">
                  <Lock aria-hidden className="size-icon-16" />
                  {intl.formatMessage(createMessages.encrypted)}
                </span>
              ) : undefined
            }
          >
            {/* A secret is write-only: the field starts empty, hides what is typed and is never offered to a password manager as stored. */}
            <Input
              type={field.secret ? 'password' : 'text'}
              autoComplete={field.secret ? 'new-password' : 'off'}
              autoCapitalize="off"
              spellCheck={false}
              className={field.mono ? 'font-mono' : undefined}
              {...form.register(`fields.${field.name}`)}
            />
          </FormField>
        ))}
        <AllowedHostsField
          value={hosts}
          onChange={(next) => {
            form.setValue('allowedHosts', [...next], {
              shouldValidate: form.formState.isSubmitted,
            });
          }}
          error={errors.allowedHosts?.message}
        />
      </section>
      <div className="lg:col-span-1">
        <ConnectionGuide service={service} />
      </div>
    </form>
  );
}

/**
 * Creates a credential in the project, in three columns: the service on the left (with a search), the form in
 * the middle and the guide to get its details on the right. Secret fields are write-only and the form lives only
 * while the dialog is open, so nothing typed outlives it. Allowed domains are required: the secret goes only there.
 */
export function CreateCredentialDialog({ open, onOpenChange, orgId, projectId, projectName, onTestConnection }: CreateCredentialDialogProps) {
  const intl = useIntl();
  const formId = useId();
  const [busy, setBusy] = useState(false);
  const [serviceId, setServiceId] = useState(SERVICES[0]?.id ?? '');
  const service = serviceById(serviceId) ?? SERVICES[0];
  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      className="max-w-screen-xl"
      title={intl.formatMessage(createMessages.title)}
      description={`${intl.formatMessage(createMessages.where, { project: projectName })} ${intl.formatMessage(createMessages.text)}`}
      footer={
        <>
          {onTestConnection === undefined ? null : (
            <Button variant="secondary" className="mr-auto" onClick={onTestConnection}>
              {intl.formatMessage(createMessages.testConnection)}
            </Button>
          )}
          <Button
            variant="ghost"
            onClick={() => {
              onOpenChange(false);
            }}
          >
            {intl.formatMessage(credentialMessages.cancel)}
          </Button>
          <Button type="submit" form={formId} loading={busy}>
            {intl.formatMessage(createMessages.save)}
          </Button>
        </>
      }
    >
      {open && service !== undefined ? (
        <CreateForm
          formId={formId}
          orgId={orgId}
          projectId={projectId}
          service={service}
          onService={setServiceId}
          onBusy={setBusy}
          onClose={() => {
            onOpenChange(false);
          }}
        />
      ) : null}
    </Dialog>
  );
}
