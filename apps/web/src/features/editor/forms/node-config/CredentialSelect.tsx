// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Banner, Button, FormField, Select, Skeleton } from '@orcon/ui';
import { useQuery } from '@tanstack/react-query';
import { Plus } from 'lucide-react';
import { useRef, useState } from 'react';
import { useIntl } from 'react-intl';

import { credentialSource, credentialsQueryOptions } from '../../api/queries';
import { useNoticeChannel } from '../../components/notice-channel';
import { credentialType } from '../../model/credential-types';
import { catalogText, localeOf, type CredentialSlot } from '../../model/node-manifests';

import { CreateCredentialDialog } from './CreateCredentialDialog';
import { configMessages } from './messages';
import { useNodeActions } from './use-node-config';

const NONE = '__none__';

export interface CredentialSelectProps {
  nodeId: string;
  slot: CredentialSlot;
  /** The name the node holds for the slot (`credentials: { <slot>: { name } }`), if any. */
  bound: string | undefined;
  t: (key: string) => string;
}

/**
 * The credential slot of a node (SPEC-014 R-ED-CRED-01..03): the credentials of a matching type the
 * person can use, with their type and last change, a way to create one without leaving the node,
 * and a notice with two ways out when the node names a credential the project does not have. Secret
 * values are never listed; the node stores the credential by name.
 */
export function CredentialSelect({ nodeId, slot, bound, t }: CredentialSelectProps) {
  const intl = useIntl();
  const language = localeOf(intl.locale);
  const notices = useNoticeChannel();
  const actions = useNodeActions(nodeId);
  const query = useQuery(credentialsQueryOptions());
  const [creating, setCreating] = useState<{ name?: string } | undefined>();
  const anchor = useRef<HTMLDivElement>(null);

  const label = t(`credential.${slot.slot}.label`);
  const note = t(`credential.${slot.slot}.note`);
  const usable = (query.data ?? []).filter((credential) => slot.types.includes(credential.type));
  const missing = bound !== undefined && query.data !== undefined && !(query.data.some((credential) => credential.name === bound && slot.types.includes(credential.type)));

  const options = [
    { value: NONE, label: intl.formatMessage(configMessages.credentialNone) },
    ...usable.map((credential) => {
      const type = credentialType(credential.type);
      return {
        value: credential.name,
        // The type is named only when the slot takes more than one: with one type it is the slot's own.
        label: intl.formatMessage(slot.types.length > 1 ? configMessages.credentialOptionTyped : configMessages.credentialOption, {
          name: credential.name,
          type: type ? catalogText(type, language, 'name') : credential.type,
          date: intl.formatDate(credential.updatedAt, { dateStyle: 'short' }),
        }),
      };
    }),
  ];

  return (
    <div ref={anchor} className="flex flex-col gap-2">
      {missing ? (
        <Banner
          tone="warning"
          title={intl.formatMessage(configMessages.credentialMissingTitle, { name: bound })}
          action={
            <>
              <Button
                variant="secondary"
                onClick={() => {
                  setCreating({ name: bound });
                }}
              >
                {intl.formatMessage(configMessages.credentialCreateMissing)}
              </Button>
              <Button
                variant="ghost"
                onClick={() => {
                  actions.setCredential(slot.slot, undefined);
                  requestAnimationFrame(() => anchor.current?.querySelector<HTMLElement>('[role="combobox"]')?.focus());
                }}
              >
                {intl.formatMessage(configMessages.credentialChooseAnother)}
              </Button>
            </>
          }
        >
          {intl.formatMessage(configMessages.credentialMissingText)}
        </Banner>
      ) : null}
      {query.isError ? (
        <Banner
          tone="danger"
          title={intl.formatMessage(configMessages.credentialLoadFailed)}
          action={
            <Button
              variant="secondary"
              onClick={() => {
                void query.refetch();
              }}
            >
              {intl.formatMessage(configMessages.credentialRetry)}
            </Button>
          }
        />
      ) : null}
      <FormField label={label} description={note} name={`/credentials/${slot.slot}`} required={slot.required === true}>
        {query.isPending ? (
          <Skeleton className="h-control w-full" />
        ) : (
          <Select
            value={bound !== undefined && !missing ? bound : NONE}
            placeholder={intl.formatMessage(configMessages.credentialPlaceholder)}
            // A long name wraps inside the field instead of overflowing it.
            className="h-auto min-h-control-target py-1"
            onValueChange={(next) => {
              actions.setCredential(slot.slot, next === NONE ? undefined : next);
            }}
            options={options}
          />
        )}
      </FormField>
      {query.isSuccess && usable.length === 0 ? <p className="text-fg-muted text-sm">{intl.formatMessage(configMessages.credentialEmpty)}</p> : null}
      <div className="flex flex-wrap items-center gap-2">
        <Button
          variant="secondary"
          onClick={() => {
            setCreating({});
          }}
        >
          <Plus aria-hidden className="size-icon-16" />
          {intl.formatMessage(configMessages.credentialCreate)}
        </Button>
      </div>
      {credentialSource.test === undefined ? <p className="text-fg-muted text-xs">{intl.formatMessage(configMessages.credentialTestUnavailable)}</p> : null}
      <CreateCredentialDialog
        open={creating !== undefined}
        onOpenChange={(open) => {
          if (!open) setCreating(undefined);
        }}
        types={slot.types}
        initialName={creating?.name}
        onCreated={(created) => {
          actions.setCredential(slot.slot, created.name);
          notices.notify('info', intl.formatMessage(configMessages.credentialCreated, { name: created.name }));
        }}
      />
    </div>
  );
}
