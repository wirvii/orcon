// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { ActiveToggle, cn, NumberFormat, ProblemAlert, RelativeTime, statusMap, type StatusKey } from '@orcon/ui';
import { Link as RouterLink } from '@tanstack/react-router';
import { Minus } from 'lucide-react';
import { defineMessages, useIntl } from 'react-intl';

import { toProblemDetails } from '../errors/to-problem-details';

import type { ListedFlow } from './flows';

const messages = defineMessages({
  name: { id: 'flowList.column.name', defaultMessage: 'Name', description: 'Column heading of the name of a flow in a list of flows.' },
  project: { id: 'flowList.column.project', defaultMessage: 'Project', description: 'Column heading of the project a flow belongs to.' },
  lastRun: { id: 'flowList.column.lastRun', defaultMessage: 'Last run', description: 'Column heading of the most recent execution of a flow: its status and when it ran.' },
  runs24h: { id: 'flowList.column.runs24h', defaultMessage: '24 h', description: 'Short column heading of the number of executions of a flow in the last 24 hours.' },
  runs24hLong: {
    id: 'flowList.runs24h.label',
    defaultMessage: 'Runs in the last 24 hours',
    description: 'Accessible name of the number of executions of a flow in the last 24 hours.',
  },
  updated: { id: 'flowList.column.updated', defaultMessage: 'Updated', description: 'Column heading of when a flow last changed.' },
  active: { id: 'flowList.column.active', defaultMessage: 'Active', description: 'Column heading of the switch that turns a flow on or off.' },
  neverRun: { id: 'flowList.lastRun.never', defaultMessage: 'Never run', description: 'Shown in place of the last execution of a flow that has never run.' },
  activeLabel: {
    id: 'flowList.toggle.label',
    defaultMessage: 'Active: {name}',
    description: 'Accessible name of the switch that turns a flow on or off. {name} is the name of the flow.',
  },
  publishFirst: {
    id: 'flowList.toggle.publishFirst',
    defaultMessage: 'Publish the flow to be able to activate it',
    description: 'Why the switch of a flow that was never published is locked.',
  },
});

/** The word and icon of the last execution of a flow: an operation of unknown outcome is shown as such, never as a success or a failure. */
function lastRunKey(flow: ListedFlow): StatusKey | undefined {
  const last = flow.lastExecution;
  if (last === undefined) return undefined;
  return last.hasUncertainOperation ? 'UNCERTAIN' : last.status;
}

/** Text color of each status family; the icon and the word carry the meaning, the color only reinforces it. */
const familyText = {
  neutral: 'text-fg-muted',
  active: 'text-status-active',
  success: 'text-status-success',
  warning: 'text-status-warning',
  danger: 'text-status-danger',
  attention: 'text-status-attention',
} as const;

/** The columns of a row on a computer: the project is one more when the list mixes projects. */
const columnsWithProject = 'md:grid-cols-[minmax(0,2.2fr)_minmax(0,1.2fr)_minmax(0,2fr)_minmax(0,0.5fr)_minmax(0,0.9fr)_9rem] md:items-center md:gap-4';
const columnsWithoutProject = 'md:grid-cols-[minmax(0,2.8fr)_minmax(0,2fr)_minmax(0,0.6fr)_minmax(0,1fr)_9rem] md:items-center md:gap-4';

export interface FlowRowsProps {
  flows: readonly ListedFlow[];
  orgSlug: string;
  /** The projects by id. When given, the list mixes projects and each row names its project. */
  projects?: ReadonlyMap<string, { name: string; handle: string }> | undefined;
  /** The handle of the project every flow belongs to, when the list is one project's. */
  projectHandle?: string | undefined;
  /** The flow whose switch is changing now. */
  busyFlowId?: string | undefined;
  onToggle: (flow: ListedFlow, active: boolean) => void;
  /** Why the last change was refused. */
  failure?: unknown;
}

/**
 * Dense rows of flows: name, last execution (icon, word and time), runs in 24 hours, last change and the Active
 * switch. A row is one link to the editor; the switch sits beside it. A flow that was never published has its
 * switch off and locked, with the reason. On a phone each row is a small card. There are no tags and no owner: the
 * contract has neither yet (SPEC-019 F-12).
 */
export function FlowRows({ flows, orgSlug, projects, projectHandle, busyFlowId, onToggle, failure }: FlowRowsProps) {
  const intl = useIntl();
  const columns = projects === undefined ? columnsWithoutProject : columnsWithProject;
  return (
    <div className="flex flex-col gap-3">
      {failure === undefined || failure === null ? null : <ProblemAlert problem={toProblemDetails(failure)} />}
      <div className="border-border bg-surface overflow-hidden rounded-lg border">
        <div aria-hidden className={cn('border-border bg-surface-sunken text-fg-muted hidden h-control border-b px-4 text-xs font-semibold md:grid', columns)}>
          <span>{intl.formatMessage(messages.name)}</span>
          {projects === undefined ? null : <span>{intl.formatMessage(messages.project)}</span>}
          <span>{intl.formatMessage(messages.lastRun)}</span>
          <span>{intl.formatMessage(messages.runs24h)}</span>
          <span>{intl.formatMessage(messages.updated)}</span>
          <span>{intl.formatMessage(messages.active)}</span>
        </div>
        <ul>
          {flows.map((flow) => {
            const project = projects?.get(flow.projectId);
            const handle = project?.handle ?? projectHandle;
            const key = lastRunKey(flow);
            const entry = key === undefined ? undefined : statusMap[key];
            const Icon = entry?.icon ?? Minus;
            const active = flow.activationState === 'ACTIVE';
            const locked = !active && (flow.activeVersion === undefined || flow.activeVersion === null);
            return (
              <li
                key={flow.id}
                className={cn(
                  'border-border hover:bg-surface-hover focus-within:bg-surface-hover relative flex items-center gap-3 border-b px-4 py-3 transition-colors last:border-b-0 md:grid md:py-2',
                  columns,
                )}
              >
                {/* On a phone the name, the project and the last run stack beside the switch; on a computer each is a column. */}
                <div className="flex min-w-0 flex-1 flex-col gap-1 md:contents">
                  <span data-truncate className="text-base min-w-0 truncate font-semibold">
                    {handle === undefined ? (
                      flow.name
                    ) : (
                      <RouterLink
                        to="/orgs/$orgSlug/projects/$projectHandle/flows/$flowId"
                        params={{ orgSlug, projectHandle: handle, flowId: flow.id }}
                        className="rounded-sm after:absolute after:inset-0"
                      >
                        {flow.name}
                      </RouterLink>
                    )}
                  </span>
                  {project === undefined ? null : (
                    <span data-truncate className="text-fg-muted min-w-0 truncate text-sm">
                      <span className="sr-only">{intl.formatMessage(messages.project)}: </span>
                      {project.name}
                    </span>
                  )}
                  <span className="text-fg-muted flex min-w-0 items-center gap-x-2 text-sm whitespace-nowrap">
                    <span className="sr-only">{intl.formatMessage(messages.lastRun)}: </span>
                    <Icon aria-hidden className={cn('size-icon-16 shrink-0', entry === undefined ? 'text-fg-subtle' : familyText[entry.family])} />
                    {entry === undefined || flow.lastExecution === undefined ? (
                      <span className="text-fg-subtle">{intl.formatMessage(messages.neverRun)}</span>
                    ) : (
                      <>
                        <span className={cn('font-semibold', familyText[entry.family])}>{intl.formatMessage(entry.label)}</span>
                        <span aria-hidden>·</span>
                        <RelativeTime value={flow.lastExecution.createdAt} className="text-fg-subtle" />
                      </>
                    )}
                  </span>
                </div>
                <span className="text-fg-muted text-sm tabular-nums max-md:hidden">
                  <span className="sr-only">{intl.formatMessage(messages.runs24hLong)}: </span>
                  <NumberFormat value={flow.executionCountLast24h ?? 0} />
                </span>
                <span className="text-fg-muted text-sm max-md:hidden">
                  <span className="sr-only">{intl.formatMessage(messages.updated)}: </span>
                  <RelativeTime value={flow.updatedAt} />
                </span>
                <span className="relative shrink-0">
                  <ActiveToggle
                    checked={active}
                    busy={busyFlowId === flow.id}
                    disabledReason={locked ? intl.formatMessage(messages.publishFirst) : undefined}
                    label={intl.formatMessage(messages.activeLabel, { name: flow.name })}
                    onCheckedChange={(next) => {
                      onToggle(flow, next);
                    }}
                  />
                </span>
              </li>
            );
          })}
        </ul>
      </div>
    </div>
  );
}
