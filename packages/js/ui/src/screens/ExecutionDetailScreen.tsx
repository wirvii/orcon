// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { useId } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { AlertDialog } from '../components/alert-dialog/AlertDialog';
import { Button } from '../components/button/Button';
import { CodeBlock } from '../components/code-block/CodeBlock';
import { DateTime } from '../components/date-time/DateTime';
import { Duration } from '../components/number-format/NumberFormat';
import { RedactedValue } from '../components/redacted-value/RedactedValue';
import { StatusBadge } from '../components/status-badge/StatusBadge';
import { UncertainResolutionPanel } from '../components/uncertain-resolution/UncertainResolutionPanel';

import { executionTimeline, uncertainResolution, type TimelineNode, type UncertainResolution } from './fixtures';
import { PageHeader } from './PageHeader';
import { ProductShell } from './ProductShell';

const messages = defineMessages({
  title: { id: 'screen.execution.title', defaultMessage: 'Order fulfilment', description: 'Name of the flow of the example execution in the detail template.' },
  flows: { id: 'screen.execution.breadcrumb', defaultMessage: 'Executions', description: 'First step of the path above an execution: the list of executions.' },
  continue: { id: 'screen.execution.continue', defaultMessage: 'Continue', description: 'Button that lets a paused execution go on.' },
  continueBlocked: {
    id: 'screen.execution.continueBlocked',
    defaultMessage: 'Resolve the operation under review first.',
    description: 'Explains why Continue is disabled: an operation with an unknown result still needs a decision.',
  },
  retry: { id: 'screen.execution.retry', defaultMessage: 'Retry failed items', description: 'Button that runs the failed items of an execution again.' },
  cancel: { id: 'screen.execution.cancel', defaultMessage: 'Cancel execution', description: 'Button that stops an execution.' },
  cancelTitle: { id: 'screen.execution.cancel.title', defaultMessage: 'Cancel this execution?', description: 'Question of the confirmation before cancelling an execution.' },
  cancelImpact: {
    id: 'screen.execution.cancel.impact',
    defaultMessage: 'The steps that have not run will not run. Steps that already ran are not undone. This cannot be reversed.',
    description: 'Explains what cancelling an execution does and that it cannot be undone.',
  },
  cancelConfirm: { id: 'screen.execution.cancel.confirm', defaultMessage: 'Cancel execution', description: 'Button that confirms cancelling the execution.' },
  timeline: { id: 'screen.execution.timeline', defaultMessage: 'Timeline', description: 'Heading of the list of steps of an execution in the order they ran.' },
  attempts: { id: 'screen.execution.attempts', defaultMessage: 'Attempts', description: 'Label of the list of attempts of one step.' },
  attemptLine: { id: 'screen.execution.attemptLine', defaultMessage: 'Attempt {number}', description: 'Name of one attempt of a step. {number} counts from 1.' },
  noAttempts: { id: 'screen.execution.noAttempts', defaultMessage: 'This step did not run.', description: 'Shown for a step that has no attempts because its path was not taken.' },
  items: {
    id: 'screen.execution.items',
    defaultMessage: '{count, plural, one {# item} other {# items}}',
    description: 'How many items a step handled.',
  },
  output: { id: 'screen.execution.output', defaultMessage: 'Output data', description: 'Label of the data a step produced.' },
  outputName: { id: 'screen.execution.outputName', defaultMessage: 'Output of {name}', description: 'Accessible name of the block with the data of a step. {name} is the step.' },
  summary: { id: 'screen.execution.summary', defaultMessage: 'Execution', description: 'Label before the execution id.' },
});

export interface ExecutionDetailScreenProps {
  timeline?: readonly TimelineNode[];
  /** The decision panel for an operation whose result is unknown. Omit when nothing needs one. */
  resolution?: UncertainResolution;
  /** The person may read execution data (the `execution:read-data` permission). Without it data is shown as hidden. */
  canReadData?: boolean;
  onContinue?: () => void;
  onRetry?: () => void;
  onCancel?: () => void;
  onResolve?: (actionId: string) => void;
}

/**
 * S5: one execution. The timeline lists the steps with their attempts and state;
 * an operation whose result is unknown gets a decision panel (never offered a
 * primary "repeat"); data a person may not read shows as hidden, never as empty.
 */
export function ExecutionDetailScreen({
  timeline = executionTimeline,
  resolution = uncertainResolution,
  canReadData = true,
  onContinue,
  onRetry,
  onCancel,
  onResolve,
}: ExecutionDetailScreenProps) {
  const intl = useIntl();
  const hintId = useId();
  const needsDecision = timeline.some((node) => node.status === 'UNCERTAIN') && resolution !== undefined;
  const overall = timeline.some((node) => node.status === 'UNCERTAIN') ? 'UNCERTAIN' : 'SUCCEEDED';

  return (
    <ProductShell current="executions">
      <PageHeader
        title={intl.formatMessage(messages.title)}
        breadcrumb={[{ label: intl.formatMessage(messages.flows), href: '#executions' }, { label: 'ex-4817' }]}
        description={
          <span className="flex flex-wrap items-center gap-x-3 gap-y-1">
            <StatusBadge status={overall} />
            <span>
              {intl.formatMessage(messages.summary)} <span className="font-mono text-mono">ex-4817</span>
            </span>
          </span>
        }
        actions={
          <>
            <Button variant="primary" disabled={needsDecision} aria-describedby={needsDecision ? hintId : undefined} onClick={onContinue}>
              {intl.formatMessage(messages.continue)}
            </Button>
            <Button variant="secondary" onClick={onRetry}>
              {intl.formatMessage(messages.retry)}
            </Button>
            <AlertDialog
              trigger={<Button variant="ghost">{intl.formatMessage(messages.cancel)}</Button>}
              title={intl.formatMessage(messages.cancelTitle)}
              impact={intl.formatMessage(messages.cancelImpact)}
              confirmLabel={intl.formatMessage(messages.cancelConfirm)}
              onConfirm={() => onCancel?.()}
            />
          </>
        }
      />
      {needsDecision ? (
        <p id={hintId} className="text-fg-muted pb-3 text-sm">
          {intl.formatMessage(messages.continueBlocked)}
        </p>
      ) : null}
      <div className="flex flex-col gap-6 lg:flex-row lg:items-start">
        <section aria-labelledby="timeline-heading" className="min-w-0 flex-1">
          <h2 id="timeline-heading" className="text-lg font-semibold wrap-anywhere">
            {intl.formatMessage(messages.timeline)}
          </h2>
          <ol className="mt-3 flex flex-col gap-3">
            {timeline.map((node) => (
              <li key={node.id} data-timeline-node={node.id} className="bg-surface border-border flex flex-col gap-2 rounded-lg border p-3">
                <div className="flex flex-wrap items-center gap-2">
                  <h3 className="font-semibold wrap-anywhere">{node.name}</h3>
                  <StatusBadge status={node.status} />
                  <span className="text-fg-muted text-sm">{intl.formatMessage(messages.items, { count: node.items })}</span>
                </div>
                {node.attempts.length === 0 ? (
                  <p className="text-fg-muted text-sm">{intl.formatMessage(messages.noAttempts)}</p>
                ) : (
                  <ul aria-label={intl.formatMessage(messages.attempts)} className="flex flex-col gap-1">
                    {node.attempts.map((attempt) => (
                      <li key={attempt.attempt} className="flex flex-wrap items-center gap-2 text-sm">
                        <span className="font-medium">{intl.formatMessage(messages.attemptLine, { number: attempt.attempt })}</span>
                        <StatusBadge status={attempt.status} />
                        <span className="text-fg-muted">
                          <DateTime value={attempt.startedAt} timeOnly />
                        </span>
                        <span className="text-fg-muted">
                          <Duration ms={attempt.durationMs} />
                        </span>
                      </li>
                    ))}
                  </ul>
                )}
                {node.output ? (
                  <div className="flex flex-col gap-1">
                    <p className="text-fg-muted text-sm font-medium">{intl.formatMessage(messages.output)}</p>
                    {canReadData ? (
                      <CodeBlock code={node.output} label={intl.formatMessage(messages.outputName, { name: node.name })} copyable />
                    ) : (
                      <RedactedValue />
                    )}
                  </div>
                ) : null}
              </li>
            ))}
          </ol>
        </section>
        {needsDecision && resolution ? (
          <div className="lg:w-node-width lg:shrink-0 xl:w-node-width">
            <UncertainResolutionPanel
              nodeName={resolution.nodeName}
              manualCheck={resolution.manualCheck}
              actions={resolution.actions}
              onResolve={onResolve}
              onCancel={onCancel}
            />
          </div>
        ) : null}
      </div>
    </ProductShell>
  );
}
