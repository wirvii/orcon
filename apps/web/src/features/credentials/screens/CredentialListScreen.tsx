// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { AlertDialog, Banner, Button, Mascot, PageHeader, RelativeTime } from '@orcon/ui';
import { useQuery } from '@tanstack/react-query';
import { getRouteApi } from '@tanstack/react-router';
import { useState } from 'react';
import { useIntl } from 'react-intl';

import { credentialType, catalogLocaleOf, credentialText } from '../../../app/credential-store/credential-types';
import type { CredentialSummary } from '../../../app/credential-store/store';
import { QueryState } from '../../../app/query-state/QueryState';
import { useDeleteCredential } from '../api/mutations';
import { credentialsQueryOptions } from '../api/queries';
import { usesLocalCredentials } from '../api/store';
import { CreateCredentialDialog } from '../forms/CreateCredentialDialog';
import { RotateCredentialDialog } from '../forms/RotateCredentialDialog';
import { credentialMessages } from '../forms/messages';

const route = getRouteApi('/_session/orgs/$orgSlug/projects/$projectHandle/credentials');

/**
 * The credentials of the project: names, types and when they changed. A secret is never shown; the only
 * way to change one is to replace it. Deleting asks first, because flows that use it fail.
 */
export function CredentialListScreen() {
  const intl = useIntl();
  const { organization, project } = route.useRouteContext();
  const credentials = useQuery(credentialsQueryOptions(organization.id, project.id));
  const remove = useDeleteCredential();
  const [creating, setCreating] = useState(false);
  const [replacing, setReplacing] = useState<CredentialSummary | undefined>(undefined);
  const [deleting, setDeleting] = useState<CredentialSummary | undefined>(undefined);

  const newCredential = (
    <Button
      onClick={() => {
        setCreating(true);
      }}
    >
      {intl.formatMessage(credentialMessages.newCredential)}
    </Button>
  );

  return (
    <div className="flex flex-col gap-4">
      <PageHeader title={intl.formatMessage(credentialMessages.title)} description={intl.formatMessage(credentialMessages.description)} actions={newCredential} />
      {usesLocalCredentials() ? <Banner tone="warning" title={intl.formatMessage(credentialMessages.previewNotice)} /> : null}
      {remove.isError ? <Banner tone="danger" title={intl.formatMessage(credentialMessages.deleteFailed)} /> : null}
      <QueryState
        query={credentials}
        empty={{
          title: intl.formatMessage(credentialMessages.emptyTitle),
          description: intl.formatMessage(credentialMessages.emptyDescription),
          action: newCredential,
          illustration: <Mascot pose="happy" />,
        }}
      >
        {(items) => (
          <ul aria-label={intl.formatMessage(credentialMessages.title)} className="border-border flex flex-col rounded-lg border">
            {items.map((credential) => (
              <CredentialRow
                key={credential.id}
                credential={credential}
                onReplace={() => {
                  setReplacing(credential);
                }}
                onDelete={() => {
                  setDeleting(credential);
                }}
              />
            ))}
          </ul>
        )}
      </QueryState>

      <CreateCredentialDialog open={creating} onOpenChange={setCreating} projectId={project.id} />
      <RotateCredentialDialog
        credential={replacing}
        onOpenChange={(open) => {
          if (!open) setReplacing(undefined);
        }}
      />
      <AlertDialog
        open={deleting !== undefined}
        onOpenChange={(open) => {
          if (!open) setDeleting(undefined);
        }}
        title={intl.formatMessage(credentialMessages.deleteTitle, { name: deleting?.name ?? '' })}
        impact={intl.formatMessage(credentialMessages.deleteImpact)}
        confirmLabel={intl.formatMessage(credentialMessages.deleteConfirm)}
        cancelLabel={intl.formatMessage(credentialMessages.cancel)}
        destructive
        onConfirm={() => {
          if (deleting !== undefined) remove.mutate(deleting.id);
          setDeleting(undefined);
        }}
      />
    </div>
  );
}

function CredentialRow({ credential, onReplace, onDelete }: { credential: CredentialSummary; onReplace: () => void; onDelete: () => void }) {
  const intl = useIntl();
  const language = catalogLocaleOf(intl.locale);
  const type = credentialType(credential.type);
  const typeName = type === undefined ? credential.type : credentialText(type, language, 'name');
  return (
    <li className="border-border flex flex-col gap-2 border-b p-4 last:border-b-0 md:flex-row md:items-center md:justify-between md:gap-4">
      <div className="flex min-w-0 flex-col gap-1">
        <span className="font-medium wrap-anywhere">{credential.name}</span>
        <span className="text-fg-muted text-sm">
          {typeName}
          {' · '}
          {intl.formatMessage(credentialMessages.secretSet)}
          {' · '}
          {intl.formatMessage(credentialMessages.changed)} <RelativeTime value={credential.updatedAt} />
        </span>
      </div>
      <div className="flex shrink-0 gap-2">
        <Button variant="ghost" onClick={onReplace}>
          {intl.formatMessage(credentialMessages.replace)}
        </Button>
        <Button variant="ghost" onClick={onDelete}>
          {intl.formatMessage(credentialMessages.remove)}
        </Button>
      </div>
    </li>
  );
}
