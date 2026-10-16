// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Banner, Button, FormField, Select, Skeleton } from '@orcon/ui';
import { useQuery } from '@tanstack/react-query';
import { KeyRound, Plus } from 'lucide-react';
import { useRef, useState } from 'react';
import { useIntl } from 'react-intl';

import { credentialsQueryOptions } from '../../../../app/credentials/api/queries';
import { CreateCredentialDialog } from '../../../../app/credentials/forms/CreateCredentialDialog';
import { credentialMessages } from '../../../../app/credentials/model/messages';
import { HTTP_CONNECTOR, SERVICES, serviceOfConnector } from '../../../../app/credentials/model/services';
import { useNoticeChannel } from '../../components/notice-channel';
import { type CredentialSlot } from '../../model/node-manifests';

import { configMessages } from './messages';
import { useNodeActions } from './use-node-config';
import { useProjectScope } from './use-project-scope';

const NONE = '__none__';

export interface CredentialSelectProps {
  nodeId: string;
  slot: CredentialSlot;
  /** The identifier of the credential the node holds for the slot, if any. */
  bound: string | undefined;
  label: string;
  note: string;
}

/**
 * The credential slot of a node (SPEC-014 R-ED-CRED-01..03): the stored credentials of the project that fit the
 * step, read from the credential service, a way to create one without leaving the node, and a notice with two ways
 * out when the node names a credential the project does not have. Secret values are never listed; the node keeps
 * the identifier of the credential, which is what the engine reads when the step runs.
 */
export function CredentialSelect({ nodeId, slot, bound, label, note }: CredentialSelectProps) {
  const intl = useIntl();
  const notices = useNoticeChannel();
  const actions = useNodeActions(nodeId);
  const scope = useProjectScope();
  const query = useQuery({
    ...credentialsQueryOptions(scope?.orgId ?? '', scope?.projectId ?? ''),
    enabled: scope !== undefined,
  });
  const [creating, setCreating] = useState(false);
  const anchor = useRef<HTMLDivElement>(null);

  const usable = (query.data ?? []).filter((credential) => slot.types.includes(credential.type));
  const chosen = bound === undefined ? undefined : usable.find((credential) => credential.id === bound);
  const missing = bound !== undefined && query.data !== undefined && chosen === undefined;

  const typeName = (type: string): string => {
    const own = serviceOfConnector(type);
    if (own !== undefined) return intl.formatMessage(own.name);
    return type === HTTP_CONNECTOR ? intl.formatMessage(credentialMessages.connectorHttp) : type;
  };

  const options = [
    { value: NONE, label: intl.formatMessage(configMessages.credentialNone) },
    ...usable.map((credential) => ({
      value: credential.id,
      label: intl.formatMessage(slot.types.length > 1 ? configMessages.credentialOptionTyped : configMessages.credentialOption, {
        name: credential.name,
        type: typeName(credential.type),
        date: intl.formatDate(credential.updatedAt, { dateStyle: 'short' }),
      }),
    })),
  ];
  const services = SERVICES.filter((service) => slot.types.includes(service.connector));

  return (
    <div ref={anchor} className="flex flex-col gap-2">
      {missing ? (
        <Banner
          tone="warning"
          title={intl.formatMessage(configMessages.credentialMissingTitle)}
          action={
            <Button
              variant="secondary"
              onClick={() => {
                actions.setCredential(slot.parameter, undefined);
                requestAnimationFrame(() => anchor.current?.querySelector<HTMLElement>('[role="combobox"]')?.focus());
              }}
            >
              {intl.formatMessage(configMessages.credentialChooseAnother)}
            </Button>
          }
        >
          {intl.formatMessage(configMessages.credentialMissingText)}
        </Banner>
      ) : null}
      {scope === undefined || query.isError ? (
        <Banner
          tone="danger"
          title={intl.formatMessage(configMessages.credentialLoadFailed)}
          action={
            query.isError ? (
              <Button
                variant="secondary"
                onClick={() => {
                  void query.refetch();
                }}
              >
                {intl.formatMessage(configMessages.credentialRetry)}
              </Button>
            ) : undefined
          }
        />
      ) : null}
      {chosen ? (
        <div data-credential-card role="group" aria-label={intl.formatMessage(configMessages.credentialCard, { name: chosen.name, type: typeName(chosen.type) })} className="border-border bg-surface flex items-center gap-3 rounded-md border p-3">
          <span aria-hidden className="bg-surface-sunken text-fg-muted grid size-10 shrink-0 place-items-center rounded-md">
            <KeyRound className="size-icon-20" />
          </span>
          <span className="flex min-w-0 flex-col">
            <span className="truncate font-semibold">{chosen.name}</span>
            <span className="text-fg-muted truncate text-sm">{typeName(chosen.type)}</span>
          </span>
        </div>
      ) : null}
      <FormField label={label} description={note} name={`/${['parameters', ...slot.parameter].join('/')}`} required={slot.required === true}>
        {query.isPending && scope !== undefined ? (
          <Skeleton className="h-control w-full" />
        ) : (
          <Select
            value={chosen === undefined ? NONE : chosen.id}
            placeholder={intl.formatMessage(configMessages.credentialPlaceholder)}
            // A long name wraps inside the field instead of overflowing it.
            className="h-auto min-h-control-target py-1"
            onValueChange={(next) => {
              actions.setCredential(slot.parameter, next === NONE ? undefined : next);
            }}
            options={options}
          />
        )}
      </FormField>
      {query.isSuccess && usable.length === 0 ? <p className="text-fg-muted text-sm">{intl.formatMessage(configMessages.credentialEmpty)}</p> : null}
      {scope === undefined ? null : (
        <div className="flex flex-wrap items-center gap-2">
          <Button
            variant="secondary"
            onClick={() => {
              setCreating(true);
            }}
          >
            <Plus aria-hidden className="size-icon-16" />
            {intl.formatMessage(configMessages.credentialCreate)}
          </Button>
        </div>
      )}
      {scope === undefined ? null : (
        <CreateCredentialDialog
          open={creating}
          onOpenChange={setCreating}
          orgId={scope.orgId}
          projectId={scope.projectId}
          projectName={scope.projectName}
          services={services}
          onCreated={(created) => {
            actions.setCredential(slot.parameter, created.id);
            notices.notify('info', intl.formatMessage(configMessages.credentialCreated, { name: created.name }));
          }}
        />
      )}
    </div>
  );
}
