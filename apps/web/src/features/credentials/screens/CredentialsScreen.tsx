// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { AlertDialog, Banner, Button, Mascot, PageHeader } from '@orcon/ui';
import { useQuery } from '@tanstack/react-query';
import { getRouteApi } from '@tanstack/react-router';
import { Plus } from 'lucide-react';
import { useState } from 'react';
import { useIntl } from 'react-intl';

import { QueryState } from '../../../app/query-state/QueryState';
import { useDeleteCredential } from '../api/mutations';
import { credentialsQueryOptions, serviceIconsQueryOptions } from '../api/queries';
import type { Credential } from '../model/credential';
import { CredentialRow } from '../components/CredentialRow';
import { CreateCredentialDialog } from '../forms/CreateCredentialDialog';
import { EditHostsDialog } from '../forms/EditHostsDialog';
import { ReplaceSecretDialog } from '../forms/ReplaceSecretDialog';
import { credentialMessages } from '../model/messages';

const route = getRouteApi('/_session/orgs/$orgSlug/projects/$projectHandle/credentials');

/**
 * The credentials of the project: name, service, the domains each key may go to and when it changed. A secret is
 * never shown, only that one is set; the way to change one is to replace it. Columns the server cannot fill yet
 * (where it is used, when it was last used, health, expiry) are not drawn rather than drawn empty. Deleting asks
 * first, because flows that use the credential fail.
 */
export function CredentialsScreen() {
  const intl = useIntl();
  const { organization, project } = route.useRouteContext();
  const credentials = useQuery(credentialsQueryOptions(organization.id, project.id));
  const icons = useQuery(serviceIconsQueryOptions(organization.id, project.id));
  const remove = useDeleteCredential();
  const [creating, setCreating] = useState(false);
  const [replacing, setReplacing] = useState<Credential | undefined>(undefined);
  const [editingHosts, setEditingHosts] = useState<Credential | undefined>(undefined);
  const [deleting, setDeleting] = useState<Credential | undefined>(undefined);

  const create = (
    <Button
      onClick={() => {
        setCreating(true);
      }}
    >
      <Plus aria-hidden className="size-icon-16" />
      {intl.formatMessage(credentialMessages.newCredential)}
    </Button>
  );

  return (
    <div className="flex flex-col gap-4">
      <PageHeader headingLevel={2} title={intl.formatMessage(credentialMessages.title)} description={intl.formatMessage(credentialMessages.description)} actions={create} />
      {remove.isError ? <Banner tone="danger" title={intl.formatMessage(credentialMessages.deleteFailed)} /> : null}
      <QueryState
        query={credentials}
        unavailable={{
          title: intl.formatMessage(credentialMessages.unavailableTitle),
          description: intl.formatMessage(credentialMessages.unavailableText),
        }}
        empty={{
          title: intl.formatMessage(credentialMessages.emptyTitle),
          description: intl.formatMessage(credentialMessages.emptyDescription),
          action: create,
          illustration: <Mascot pose="thinking" />,
        }}
      >
        {(items) => (
          <ul aria-label={intl.formatMessage(credentialMessages.listLabel)} className="border-border flex flex-col rounded-lg border">
            {items.map((credential) => (
              <CredentialRow
                key={credential.id}
                credential={credential}
                icons={icons.data}
                onReplace={() => {
                  setReplacing(credential);
                }}
                onEditHosts={() => {
                  setEditingHosts(credential);
                }}
                onDelete={() => {
                  setDeleting(credential);
                }}
              />
            ))}
          </ul>
        )}
      </QueryState>

      <CreateCredentialDialog open={creating} onOpenChange={setCreating} orgId={organization.id} projectId={project.id} projectName={project.name} />
      <ReplaceSecretDialog
        orgId={organization.id}
        credential={replacing}
        onOpenChange={(open) => {
          if (!open) setReplacing(undefined);
        }}
      />
      <EditHostsDialog
        orgId={organization.id}
        credential={editingHosts}
        onOpenChange={(open) => {
          if (!open) setEditingHosts(undefined);
        }}
      />
      <AlertDialog
        open={deleting !== undefined}
        onOpenChange={(open) => {
          if (!open) setDeleting(undefined);
        }}
        title={intl.formatMessage(credentialMessages.deleteTitle, {
          name: deleting?.name ?? '',
        })}
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
