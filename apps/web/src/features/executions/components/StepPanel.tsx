// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { CodeBlock, PlainError, ProblemAlert, Tabs, type TabItem } from '@orcon/ui';
import type { ReactNode } from 'react';
import { useIntl } from 'react-intl';

import { toProblemDetails } from '../../../app/errors/to-problem-details';
import type { RunStep } from '../model/steps';
import type { ExecutionDataEntry } from '../model/types';

import { runMessages } from './messages';

export interface StepPanelProps {
  step: RunStep;
  /** The items of the whole run. Only the ones of this step are shown. */
  entries: readonly ExecutionDataEntry[] | undefined;
  /** The person may not read run data: the items are replaced by a sentence that says why. */
  dataHidden: boolean;
  /** Why the items could not be read, when they could not. */
  dataError?: unknown;
}

/** The items of one side of a step, one block per port. */
function Items({ step, entries, side }: { step: RunStep; entries: readonly ExecutionDataEntry[]; side: 'INPUT' | 'OUTPUT' }): ReactNode {
  const intl = useIntl();
  const mine = entries.filter((entry) => step.nodeExecutionIds.includes(entry.nodeExecutionId) && entry.side === side);
  if (mine.length === 0) return <p className="text-fg-muted">{intl.formatMessage(runMessages.noItemsAtAll)}</p>;
  return (
    <div className="flex flex-col gap-3">
      {mine.map((entry) => (
        <CodeBlock
          key={`${entry.nodeExecutionId}-${entry.side}-${entry.port}`}
          label={intl.formatMessage(side === 'INPUT' ? runMessages.inputOf : runMessages.outputOf, { node: step.name, port: entry.port })}
          code={entry.dataItems.length === 0 ? intl.formatMessage(runMessages.noItems) : JSON.stringify(entry.dataItems, null, 2)}
        />
      ))}
    </div>
  );
}

/**
 * What one step of a run did: why it failed (in plain words, with the original error beside it), the items it
 * received and produced, and how many times it was tried. A person without the permission to read run data sees
 * the statuses and a sentence in place of the items.
 */
export function StepPanel({ step, entries, dataHidden, dataError }: StepPanelProps) {
  const intl = useIntl();
  const count = (side: 'INPUT' | 'OUTPUT') =>
    (entries ?? []).filter((entry) => step.nodeExecutionIds.includes(entry.nodeExecutionId) && entry.side === side).reduce((sum, entry) => sum + entry.itemCount, 0);

  const items = (side: 'INPUT' | 'OUTPUT'): ReactNode => {
    if (dataHidden) return <p className="text-fg-muted">{intl.formatMessage(runMessages.dataHidden)}</p>;
    if (dataError !== undefined) return <ProblemAlert problem={toProblemDetails(dataError)} />;
    return <Items step={step} entries={entries ?? []} side={side} />;
  };

  const tabs: TabItem[] = [];
  if (step.failure !== undefined) {
    const { code, title, detail } = step.failure;
    tabs.push({
      value: 'error',
      label: intl.formatMessage(runMessages.stepError),
      content: (
        <PlainError
          title={title ?? intl.formatMessage(runMessages.stepFailedTitle)}
          detail={detail}
          original={code === undefined ? undefined : intl.formatMessage(runMessages.originalCode, { code, node: step.id })}
          originalLabel={intl.formatMessage(runMessages.originalError)}
          headingLevel={3}
        />
      ),
    });
  }
  tabs.push(
    { value: 'input', label: `${intl.formatMessage(runMessages.stepInput)} ${dataHidden ? '' : String(count('INPUT'))}`.trim(), content: items('INPUT') },
    { value: 'output', label: `${intl.formatMessage(runMessages.stepOutput)} ${dataHidden ? '' : String(count('OUTPUT'))}`.trim(), content: items('OUTPUT') },
    { value: 'attempts', label: intl.formatMessage(runMessages.stepAttempts), content: <p>{intl.formatMessage(runMessages.attempts, { count: step.attempts })}</p> },
  );
  // Tabs keep their own state; the key makes a new step start on its first tab (the error, when there is one).
  return <Tabs key={step.id} tabs={tabs} label={intl.formatMessage(runMessages.stepTabs)} />;
}
