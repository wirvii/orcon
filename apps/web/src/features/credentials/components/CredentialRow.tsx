// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Badge, DropdownMenu, IconButton, RelativeTime } from '@orcon/ui';
import { Globe, KeyRound, Lock, MoreHorizontal, Pencil, Trash2, TriangleAlert } from 'lucide-react';
import { useIntl } from 'react-intl';

import type { ServiceIcons } from '../model/service-icons';
import type { Credential } from '../model/credential';
import { credentialMessages } from '../model/messages';
import { HTTP_CONNECTOR } from '../model/services';

import { ServiceIcon } from './ServiceIcon';

export interface CredentialRowProps {
  credential: Credential;
  icons: ServiceIcons | undefined;
  onReplace: () => void;
  onEditHosts: () => void;
  onDelete: () => void;
}

const STATUS_TEXT = {
  NEEDS_SECRET: credentialMessages.statusNeedsSecret,
  NEEDS_REAUTH: credentialMessages.statusNeedsReauth,
  DISABLED: credentialMessages.statusDisabled,
} as const;

/**
 * One credential of the list: its name, the service it is for, where its key may go and when it changed. The
 * state of the secret is a word and an icon, never a color alone, and says only that a secret is set. A
 * credential shared from another project can be used but not changed here, so it has no menu.
 */
export function CredentialRow({ credential, icons, onReplace, onEditHosts, onDelete }: CredentialRowProps) {
  const intl = useIntl();
  const serviceName = credential.type === HTTP_CONNECTOR ? intl.formatMessage(credentialMessages.connectorHttp) : credential.type;
  const [firstHost, ...otherHosts] = credential.allowedHosts;
  const state = credential.status === 'OK' ? undefined : STATUS_TEXT[credential.status];
  return (
    <li className="border-border flex flex-wrap items-center gap-3 border-b px-4 py-3 last:border-b-0 md:flex-nowrap">
      <ServiceIcon connector={credential.type} name={serviceName} icons={icons} />
      <div className="flex min-w-0 flex-1 basis-0 flex-col gap-1">
        <span className="text-md font-semibold wrap-anywhere">{credential.name}</span>
        <span className="text-fg-muted flex flex-wrap items-center gap-x-3 gap-y-1 text-sm">
          <span>{serviceName}</span>
          {firstHost === undefined ? null : (
            <span className="flex items-center gap-1">
              <Globe aria-hidden className="size-icon-16 shrink-0" />
              <span className="sr-only">{intl.formatMessage(credentialMessages.domains)}</span>
              <span className="font-mono wrap-anywhere">{firstHost}</span>
              {otherHosts.length === 0 ? null : (
                <span>
                  {intl.formatMessage(credentialMessages.domainsMore, {
                    count: otherHosts.length,
                  })}
                </span>
              )}
            </span>
          )}
          <span>
            {intl.formatMessage(credentialMessages.changed)} <RelativeTime value={credential.updatedAt} />
          </span>
        </span>
        {credential.owned ? null : <span className="text-fg-muted text-sm">{intl.formatMessage(credentialMessages.sharedHint)}</span>}
      </div>
      <div className="flex shrink-0 flex-wrap items-center gap-2 max-md:order-last max-md:basis-full">
        {credential.owned ? null : (
          <Badge tone="neutral" icon={<KeyRound aria-hidden className="size-icon-16" />}>
            {intl.formatMessage(credentialMessages.shared)}
          </Badge>
        )}
        {state === undefined ? (
          credential.secret.set ? (
            <Badge tone="neutral" icon={<Lock aria-hidden className="size-icon-16" />}>
              {intl.formatMessage(credentialMessages.secretSet)}
            </Badge>
          ) : null
        ) : (
          <Badge tone="warning" icon={<TriangleAlert aria-hidden className="size-icon-16" />}>
            {intl.formatMessage(state)}
          </Badge>
        )}
      </div>
      {credential.owned ? (
        <DropdownMenu
          trigger={
            <IconButton
              variant="ghost"
              icon={MoreHorizontal}
              label={intl.formatMessage(credentialMessages.actions, {
                name: credential.name,
              })}
            />
          }
          items={[
            {
              type: 'item',
              label: intl.formatMessage(credentialMessages.replace),
              icon: Lock,
              onSelect: onReplace,
            },
            {
              type: 'item',
              label: intl.formatMessage(credentialMessages.editDomains),
              icon: Pencil,
              onSelect: onEditHosts,
            },
            { type: 'separator' },
            {
              type: 'item',
              label: intl.formatMessage(credentialMessages.remove),
              icon: Trash2,
              destructive: true,
              onSelect: onDelete,
            },
          ]}
        />
      ) : null}
    </li>
  );
}
