// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Banner, Button, ProblemAlert, type ProblemDetails } from '@orcon/ui';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { problemStatus, toProblemDetails } from '../../../app/errors/to-problem-details';
import { useSetFlowActivation } from '../api/mutations';
import { editorKeys, flowQueryOptions, LOCAL_ORGANIZATION } from '../api/queries';
import { editorMessages } from '../components/messages';
import { PublishDialog } from './PublishDialog';

const messages = defineMessages({
  active: { id: 'editor.activation.active', defaultMessage: 'Active', description: 'State of a flow whose triggers are on, shown as text.' },
  inactive: { id: 'editor.activation.inactive', defaultMessage: 'Inactive', description: 'State of a flow whose triggers are off, shown as text.' },
  versionShown: {
    id: 'editor.activation.version',
    defaultMessage: 'Version {version} is published',
    description: 'Says which published version new runs use.',
  },
  notPublished: { id: 'editor.activation.notPublished', defaultMessage: 'Not published yet', description: 'Says the flow has no published version, so it cannot be activated.' },
  unpublishedChanges: {
    id: 'editor.activation.unpublished',
    defaultMessage: 'Unpublished changes',
    description: 'Says the draft differs from the published version.',
  },
  activate: { id: 'editor.activation.activate', defaultMessage: 'Activate', description: 'Button that switches the automatic triggers of the flow on.' },
  deactivate: { id: 'editor.activation.deactivate', defaultMessage: 'Deactivate', description: 'Button that switches the automatic triggers of the flow off.' },
  publish: { id: 'editor.activation.publish', defaultMessage: 'Publish', description: 'Button that opens the dialog to publish the draft as a new version.' },
  changedElsewhere: {
    id: 'editor.activation.changed',
    defaultMessage: 'Nothing was changed. Reload the flow to see its newest state.',
    description: 'Shown when an activation is refused because the flow changed on the server.',
  },
  reload: { id: 'editor.activation.reload', defaultMessage: 'Reload the flow', description: 'Button that reads the newest state of the flow.' },
});

export interface PublishControlsProps {
  flowId: string;
}

/**
 * The publish and activation controls of the open flow (SPEC-014 ED10a). The state is words: the
 * activation state, the published version and whether the draft has unpublished changes. An activation
 * is based on the entity tag the state was read at; when the flow changed on the server, nothing is
 * changed and the person is offered a reload.
 */
export function PublishControls({ flowId }: PublishControlsProps) {
  const intl = useIntl();
  const queryClient = useQueryClient();
  const flow = useQuery(flowQueryOptions(flowId));
  const activation = useSetFlowActivation(flowId);
  const [publishing, setPublishing] = useState(false);
  const [failure, setFailure] = useState<ProblemDetails | undefined>(undefined);
  const [stale, setStale] = useState(false);

  if (flow.status === 'pending') return null;
  if (flow.status === 'error') return <ProblemAlert problem={toProblemDetails(flow.error)} />;

  const { flow: state, etag } = flow.data;
  const active = state.activationState === 'ACTIVE';
  // A flow with no published version cannot be activated: the server refuses it, so the button stays off.
  const published = state.activeVersion !== undefined && state.activeVersion !== null;

  const toggle = async () => {
    setFailure(undefined);
    setStale(false);
    try {
      await activation.mutateAsync({ active: !active, etag });
    } catch (error) {
      if (problemStatus(error) === 412) setStale(true);
      else setFailure(toProblemDetails(error));
    }
  };

  const version =
    state.activeVersion === undefined || state.activeVersion === null
      ? intl.formatMessage(messages.notPublished)
      : intl.formatMessage(messages.versionShown, { version: state.activeVersion });

  return (
    <div className="flex flex-col gap-2">
      <div className="flex flex-wrap items-center gap-3">
        <span className="font-medium">{intl.formatMessage(active ? messages.active : messages.inactive)}</span>
        <span className="text-fg-muted">{version}</span>
        {state.hasUnpublishedChanges ? <span className="text-fg-muted">{intl.formatMessage(messages.unpublishedChanges)}</span> : null}
        <Button variant="secondary" disabled={!published && !active} loading={activation.isPending} onClick={() => void toggle()}>
          {intl.formatMessage(active ? messages.deactivate : messages.activate)}
        </Button>
        <Button
          onClick={() => {
            setPublishing(true);
          }}
        >
          {intl.formatMessage(messages.publish)}
        </Button>
      </div>
      {stale ? (
        <Banner
          tone="warning"
          title={intl.formatMessage(editorMessages.conflictTitle)}
          action={
            <Button
              variant="secondary"
              onClick={() => {
                setStale(false);
                void queryClient.invalidateQueries({ queryKey: editorKeys.flow(LOCAL_ORGANIZATION, flowId) });
              }}
            >
              {intl.formatMessage(messages.reload)}
            </Button>
          }
        >
          {intl.formatMessage(messages.changedElsewhere)}
        </Banner>
      ) : null}
      {failure ? <ProblemAlert problem={failure} /> : null}
      <PublishDialog open={publishing} onOpenChange={setPublishing} flowId={flowId} />
    </div>
  );
}
