// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { ActiveToggle, Banner, Button, ProblemAlert, Tooltip, type ProblemDetails } from '@orcon/ui';
import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { problemStatus, toProblemDetails } from '../../../app/errors/to-problem-details';
import { useSetFlowActivation } from '../api/mutations';
import { flowQueryOptions } from '../api/queries';
import { editorMessages } from '../components/messages';
import { PublishDialog } from './PublishDialog';

const messages = defineMessages({
  versionShown: {
    id: 'editor.activation.version',
    defaultMessage: 'Version {version} is published',
    description: 'Says which published version new runs use.',
  },
  notPublished: { id: 'editor.activation.notPublished', defaultMessage: 'Not published yet', description: 'Says the flow has no published version, so it cannot be activated.' },
  switchLabel: {
    id: 'editor.activation.switchLabel',
    defaultMessage: 'Active: {name}',
    description: 'Accessible name of the switch that turns the automatic triggers of a flow on or off. {name} is the name of the flow.',
  },
  publishFirst: {
    id: 'editor.activation.publishFirst',
    defaultMessage: 'Publish a version first: only a published version can run.',
    description: 'Why the Active switch is blocked: the flow has no published version yet.',
  },
  publish: { id: 'editor.activation.publish', defaultMessage: 'Publish', description: 'Button that opens the dialog to publish the draft as a new version.' },
  publishPending: {
    id: 'editor.activation.publishPending',
    defaultMessage: 'Publish, {count, plural, one {# change} other {# changes}} not published yet',
    description: 'Accessible name of the Publish button when the draft has changes the published version does not have. {count} is how many.',
  },
  changedElsewhere: {
    id: 'editor.activation.changed',
    defaultMessage: 'Nothing was changed. The flow changed somewhere else and now shows its newest state.',
    description: 'Shown when an activation is refused because the flow changed on the server; the flow was read again.',
  },
});

export interface PublishControlsProps {
  flowId: string;
  /** The name of the flow, for the accessible name of the switch. */
  flowName?: string | undefined;
  /** How many changes the draft has that the published version does not; absent when nothing is published or nothing differs. */
  pending?: number | undefined;
}

/**
 * Active and Publish together (SPEC-019 R-UX-EDT-01, SPEC-014 ED10a). Active says whether the flow listens for
 * its triggers; Publish says which version runs and carries the number of changes not yet published. An
 * activation is based on the entity tag the state was read at: when the flow changed on the server, nothing is
 * changed and the person is offered a reload. A refusal is shown under the controls, over the canvas.
 */
export function PublishControls({ flowId, flowName = '', pending }: PublishControlsProps) {
  const intl = useIntl();
  const flow = useQuery(flowQueryOptions(flowId));
  const activation = useSetFlowActivation(flowId);
  const [publishing, setPublishing] = useState(false);
  const [failure, setFailure] = useState<ProblemDetails | undefined>(undefined);
  const [stale, setStale] = useState(false);
  const [needsPublish, setNeedsPublish] = useState(false);

  if (flow.status === 'pending') return null;
  if (flow.status === 'error') return <ProblemAlert problem={toProblemDetails(flow.error)} />;

  const { flow: state, etag } = flow.data;
  const active = state.activationState === 'ACTIVE';
  // A flow with no published version cannot be activated: the server refuses it, so the switch says why and stays off.
  const published = state.activeVersion !== undefined && state.activeVersion !== null;

  const toggle = async () => {
    setFailure(undefined);
    setStale(false);
    setNeedsPublish(false);
    try {
      await activation.mutateAsync({ active: !active, etag });
    } catch (error) {
      const status = problemStatus(error);
      // 412: the flow changed elsewhere (the mutation reads it again). 409: the flow has no published version.
      if (status === 412) setStale(true);
      else if (status === 409) setNeedsPublish(true);
      else setFailure(toProblemDetails(error));
    }
  };

  const version = published ? intl.formatMessage(messages.versionShown, { version: state.activeVersion }) : intl.formatMessage(messages.notPublished);
  const count = pending !== undefined && pending > 0 ? pending : undefined;

  return (
    <div className="relative flex items-center gap-3" data-publish-controls>
      <ActiveToggle
        checked={active}
        busy={activation.isPending}
        label={intl.formatMessage(messages.switchLabel, { name: flowName })}
        disabledReason={!published && !active ? intl.formatMessage(messages.publishFirst) : undefined}
        onCheckedChange={() => void toggle()}
      />
      <Tooltip content={version}>
        <Button
          aria-label={count === undefined ? undefined : intl.formatMessage(messages.publishPending, { count })}
          onClick={() => {
            setPublishing(true);
          }}
        >
          {intl.formatMessage(messages.publish)}
          {count === undefined ? null : (
            <span aria-hidden data-pending-count className="bg-action-primary-fg text-action-primary inline-flex min-w-5 items-center justify-center rounded-full px-1 text-xs font-semibold">
              {count}
            </span>
          )}
        </Button>
      </Tooltip>
      {stale || needsPublish || failure ? (
        <div className="absolute end-0 top-full z-40 mt-2 flex w-canvas-palette-w max-w-full flex-col gap-2">
          {stale ? (
            <Banner tone="warning" title={intl.formatMessage(editorMessages.conflictTitle)}>
              {intl.formatMessage(messages.changedElsewhere)}
            </Banner>
          ) : null}
          {needsPublish ? <Banner tone="info" title={intl.formatMessage(messages.publishFirst)} /> : null}
          {failure ? <ProblemAlert problem={failure} /> : null}
        </div>
      ) : null}
      <PublishDialog open={publishing} onOpenChange={setPublishing} flowId={flowId} />
    </div>
  );
}
