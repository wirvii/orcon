// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { DateTime, formatDuration, StatusBadge, type StatusKey } from '@orcon/ui';
import type { ReactNode } from 'react';
import { useIntl } from 'react-intl';

import { runMessages } from './messages';

export interface RunHeaderProps {
  /** The flow the run belongs to, by name. */
  flowName: string;
  /** The end of the identifier, which tells runs of one flow apart. */
  tail: string;
  status: StatusKey;
  startedAt: string | undefined;
  durationMs: number | undefined;
  /** How the run started, already in words. */
  mode: string;
  /** The version of the flow: its number, or "Draft" for a run of a draft. */
  version: string;
  projectName?: string | undefined;
  /** The whole identifier, which the search box takes. */
  id: string;
  isRetry: boolean;
  /** 1 when the run fills the page; 2 when it sits beside a list that has its own heading. */
  headingLevel: 1 | 2;
  /** The one main action of the run's state: a retry for a failed run. A run that needs review has none here. */
  action?: ReactNode;
  /** A way back to the list, shown on a phone where the list and the run are on different screens. */
  back?: ReactNode;
}

/**
 * The top of one run: the flow and the number of the run, its status, and when, for how long, how and on which
 * version it ran. The main action depends on the status and is given by the screen; a run that needs review never
 * offers a retry as its main action.
 */
export function RunHeader({ headingLevel, flowName, tail, status, startedAt, durationMs, mode, version, projectName, id, isRetry, action, back }: RunHeaderProps) {
  const intl = useIntl();
  const Heading = headingLevel === 1 ? 'h1' : 'h2';
  return (
    <header className="flex flex-col gap-3">
      {back}
      <div className="flex flex-wrap items-center gap-3">
        <StatusBadge status={status} />
        {/* On a phone the name takes the first row, so it is never squeezed between the status and the action. */}
        <Heading className="text-lg min-w-0 flex-1 font-semibold wrap-anywhere max-md:order-first max-md:basis-full">
          {flowName} <span className="text-fg-muted font-normal">{intl.formatMessage(runMessages.runNumber, { id: tail })}</span>
        </Heading>
        {action}
      </div>
      <dl className="text-fg-muted flex flex-wrap gap-x-6 gap-y-1 text-sm">
        <div className="flex gap-1">
          <dt>{intl.formatMessage(runMessages.started)}</dt>
          <dd className="text-fg font-medium">{startedAt === undefined ? intl.formatMessage(runMessages.notStarted) : <DateTime value={startedAt} dateStyle="medium" timeStyle="medium" />}</dd>
        </div>
        <div className="flex gap-1">
          <dt>{intl.formatMessage(runMessages.duration)}</dt>
          <dd className="text-fg font-medium">{durationMs === undefined ? intl.formatMessage(runMessages.stillRunning) : formatDuration(durationMs, intl.locale)}</dd>
        </div>
        <div className="flex gap-1">
          <dt>{intl.formatMessage(runMessages.trigger)}</dt>
          <dd className="text-fg font-medium">{mode}</dd>
        </div>
        <div className="flex gap-1">
          <dt>{intl.formatMessage(runMessages.versionLabel)}</dt>
          <dd className="text-fg font-medium">{version}</dd>
        </div>
        {projectName === undefined ? null : (
          <div className="flex gap-1">
            <dt>{intl.formatMessage(runMessages.projectLabel)}</dt>
            <dd className="text-fg font-medium">{projectName}</dd>
          </div>
        )}
        <div className="flex min-w-0 gap-1">
          <dt>{intl.formatMessage(runMessages.runId)}</dt>
          <dd className="text-fg font-mono wrap-anywhere">{id}</dd>
        </div>
        {isRetry ? <div className="text-fg font-medium">{intl.formatMessage(runMessages.retryOf)}</div> : null}
      </dl>
    </header>
  );
}
