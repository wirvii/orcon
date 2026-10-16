// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Banner, Button, Dialog, ProblemAlert, Skeleton } from '@orcon/ui';
import { useQuery } from '@tanstack/react-query';
import { useId, useState } from 'react';
import { useIntl } from 'react-intl';

import { problemStatus, toProblemDetails } from '../../../app/errors/to-problem-details';
import { useUpdateAllowedHosts } from '../../../app/credentials/api/mutations';
import { credentialQueryOptions } from '../../../app/credentials/api/queries';
import type { Credential } from '../../../app/credentials/model/credential';
import { AllowedHostsField } from '../../../app/credentials/components/AllowedHostsField';

import { changeMessages, credentialMessages, hostMessages } from '../../../app/credentials/model/messages';

export interface EditHostsDialogProps {
  orgId: string;
  /** The credential whose allowed domains change; the dialog is closed when there is none. */
  credential: Credential | undefined;
  onOpenChange: (open: boolean) => void;
}

/** Changes the sites a credential's secret may be sent to. The list is read when the dialog opens and saved with its entity tag. */
export function EditHostsDialog({ orgId, credential, onOpenChange }: EditHostsDialogProps) {
  const intl = useIntl();
  const formId = useId();
  const [busy, setBusy] = useState(false);
  return (
    <Dialog
      open={credential !== undefined}
      onOpenChange={onOpenChange}
      title={intl.formatMessage(hostMessages.editTitle, {
        name: credential?.name ?? '',
      })}
      description={intl.formatMessage(hostMessages.editText)}
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
            {intl.formatMessage(hostMessages.editSave)}
          </Button>
        </>
      }
    >
      {credential === undefined ? null : (
        <HostsLoader
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

function HostsLoader({ orgId, credentialId, formId, onBusy, onDone }: { orgId: string; credentialId: string; formId: string; onBusy: (busy: boolean) => void; onDone: () => void }) {
  const intl = useIntl();
  const loaded = useQuery(credentialQueryOptions(orgId, credentialId));
  if (loaded.status === 'pending') return <Skeleton role="status" aria-label={intl.formatMessage(changeMessages.loading)} className="h-16 w-full" />;
  if (loaded.status === 'error') return <Banner tone="danger" title={intl.formatMessage(changeMessages.loadFailed)} />;
  return <HostsForm formId={formId} credentialId={credentialId} etag={loaded.data.etag} initial={loaded.data.credential.allowedHosts} onBusy={onBusy} onDone={onDone} />;
}

function HostsForm({
  formId,
  credentialId,
  etag,
  initial,
  onBusy,
  onDone,
}: {
  formId: string;
  credentialId: string;
  etag: string;
  initial: readonly string[];
  onBusy: (busy: boolean) => void;
  onDone: () => void;
}) {
  const intl = useIntl();
  const update = useUpdateAllowedHosts();
  const [hosts, setHosts] = useState<readonly string[]>(initial);
  const [empty, setEmpty] = useState(false);
  const [failure, setFailure] = useState<unknown>(undefined);

  const submit = async () => {
    setFailure(undefined);
    if (hosts.length === 0) {
      setEmpty(true);
      return;
    }
    onBusy(true);
    try {
      await update.mutateAsync({ credentialId, etag, allowedHosts: hosts });
      update.reset();
      onDone();
    } catch (error) {
      update.reset();
      setFailure(error);
    } finally {
      onBusy(false);
    }
  };

  return (
    <form
      id={formId}
      noValidate
      onSubmit={(event) => {
        event.preventDefault();
        void submit();
      }}
      className="flex flex-col gap-3"
    >
      {failure === undefined ? null : problemStatus(failure) === 412 ? (
        <Banner tone="warning" title={intl.formatMessage(changeMessages.stale)} />
      ) : (
        <ProblemAlert problem={toProblemDetails(failure)} />
      )}
      <AllowedHostsField
        value={hosts}
        onChange={(next) => {
          setHosts(next);
          setEmpty(false);
        }}
        error={empty ? intl.formatMessage(hostMessages.required) : undefined}
      />
    </form>
  );
}
