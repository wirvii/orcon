// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { cn, NumberFormat } from '@orcon/ui';
import { Activity, Clock, ShieldQuestion, XCircle, Zap, type LucideIcon } from 'lucide-react';
import type { ReactNode } from 'react';
import { defineMessages, useIntl } from 'react-intl';

const messages = defineMessages({
  label: { id: 'overview.stats.label', defaultMessage: 'Activity of the organization', description: 'Accessible name of the group of five figures at the top of the home page.' },
  activeFlows: { id: 'overview.stats.activeFlows', defaultMessage: 'Active flows', description: 'Name of the figure that counts the flows that are active.' },
  activeOf: { id: 'overview.stats.activeOf', defaultMessage: 'of {total}', description: 'Small text after the number of active flows. {total} is how many flows there are.' },
  projects: {
    id: 'overview.stats.projects',
    defaultMessage: '{count, plural, one {# project} other {# projects}}',
    description: 'Detail under the figure of active flows: how many projects the flows belong to.',
  },
  runs: { id: 'overview.stats.runs', defaultMessage: 'Runs · 24 h', description: 'Name of the figure that counts the executions of the last 24 hours.' },
  runsByHour: {
    id: 'overview.stats.runsByHour',
    defaultMessage: 'Runs in each of the last 24 hours, the most recent hour last',
    description: 'Accessible name of the small bar chart of the executions of the last 24 hours.',
  },
  failed: { id: 'overview.stats.failed', defaultMessage: 'Failed · 24 h', description: 'Name of the figure that counts the executions of the last 24 hours that failed.' },
  failedShare: {
    id: 'overview.stats.failedShare',
    defaultMessage: '{share} of the runs',
    description: 'Detail under the figure of failed executions. {share} is a percentage, for example 0.2%.',
  },
  noFailures: { id: 'overview.stats.noFailures', defaultMessage: 'Nothing failed', description: 'Detail under the figure of failed executions when none failed.' },
  review: { id: 'overview.stats.review', defaultMessage: 'Needs review', description: 'Name of the figure that counts the executions whose outcome is unknown and need a person to review them.' },
  reviewDetail: {
    id: 'overview.stats.reviewDetail',
    defaultMessage: 'We do not know if the operation went through',
    description: 'Detail under the figure of executions to review: why they wait for a person.',
  },
  reviewNone: { id: 'overview.stats.reviewNone', defaultMessage: 'Nothing is waiting for you', description: 'Detail under the figure of executions to review when there are none.' },
  timeSaved: { id: 'overview.stats.timeSaved', defaultMessage: 'Time saved · {days} days', description: 'Name of the figure that estimates the time saved by the executions of the last days. {days} is how many days.' },
  hours: { id: 'overview.stats.hours', defaultMessage: '{hours} h', description: 'Time saved in whole hours, for example 31 h.' },
  minutes: { id: 'overview.stats.minutes', defaultMessage: '{minutes} min', description: 'Time saved in minutes when it is less than an hour, for example 45 min.' },
  timeSavedNote: {
    id: 'overview.stats.timeSavedNote',
    defaultMessage: 'Estimated with {minutes} min per task',
    description: 'Detail under the time saved: the minutes per task the organization uses for the estimate.',
  },
});

export interface ActivityFigures {
  activeFlowCount: number;
  flowCount: number;
  projectCount: number;
  executionsLast24h: { total: number; failed: number; uncertain: number; byHour: readonly number[] };
  timeSaved: { windowDays: number; minutesPerExecution: number; minutes: number };
}

function Card({ icon: Icon, label, attention = false, wide = false, children }: { icon: LucideIcon; label: string; attention?: boolean; wide?: boolean; children: ReactNode }) {
  return (
    <li
      className={cn(
        'border-border bg-surface flex flex-col gap-2 rounded-lg border p-3 md:p-4',
        attention && 'border-status-attention',
        wide && 'col-span-2 md:col-span-1',
      )}
    >
      <span className="text-fg-subtle flex items-center gap-2 text-xs font-medium">
        <Icon aria-hidden className="size-icon-16 shrink-0" />
        {label}
      </span>
      {children}
    </li>
  );
}

function Value({ children, attention = false }: { children: ReactNode; attention?: boolean }) {
  return <span className={cn('text-xl leading-8 font-bold tabular-nums md:text-2xl', attention && 'text-status-attention')}>{children}</span>;
}

function Detail({ children }: { children: ReactNode }) {
  return <span className="text-fg-subtle text-xs">{children}</span>;
}

/** The last 24 hours as small bars, the most recent hour last. The picture is described once, for assistive technology; the figure beside it is the number. */
function HourlyBars({ byHour, label }: { byHour: readonly number[]; label: string }) {
  const peak = Math.max(1, ...byHour);
  // Six heights on the spacing scale, from the lowest bar to the tallest: the scale allows no free height.
  const heights = ['h-1', 'h-2', 'h-3', 'h-4', 'h-5', 'h-6'] as const;
  return (
    <span role="img" aria-label={label} className="flex h-6 items-end">
      {byHour.map((count, hour) => (
        // The transparent side borders leave a thin gap between bars without spacing that is off the scale.
        <span key={hour} className="flex flex-1 items-end border-x border-transparent">
          <span className={`bg-status-success-bg border-status-success w-full rounded-t-sm border-t-2 ${heights[Math.min(5, Math.floor((count / peak) * 5.999))] ?? 'h-1'}`} />
        </span>
      ))}
    </span>
  );
}

/**
 * The five figures at the top of the home page: active flows, runs of the last 24 hours with one bar per hour,
 * failed runs, runs that need a person to review them, and the time saved. They count only the projects the
 * person can read. Percentages and comparisons that the server does not send (against yesterday, the flow with
 * most failures) are not shown.
 */
export function ActivityStats({ figures }: { figures: ActivityFigures }) {
  const intl = useIntl();
  const { executionsLast24h: runs, timeSaved } = figures;
  const failedShare = runs.total === 0 ? 0 : runs.failed / runs.total;
  const hours = Math.floor(timeSaved.minutes / 60);
  return (
    <ul aria-label={intl.formatMessage(messages.label)} className="grid grid-cols-2 gap-2 md:grid-cols-5 md:gap-3">
      <Card icon={Zap} label={intl.formatMessage(messages.activeFlows)}>
        <Value>
          <NumberFormat value={figures.activeFlowCount} />
          <small className="text-fg-subtle ml-1 text-sm font-medium">{intl.formatMessage(messages.activeOf, { total: figures.flowCount })}</small>
        </Value>
        <Detail>{intl.formatMessage(messages.projects, { count: figures.projectCount })}</Detail>
      </Card>
      <Card icon={Activity} label={intl.formatMessage(messages.runs)}>
        <Value>
          <NumberFormat value={runs.total} />
        </Value>
        <HourlyBars byHour={runs.byHour} label={intl.formatMessage(messages.runsByHour)} />
      </Card>
      <Card icon={XCircle} label={intl.formatMessage(messages.failed)}>
        <Value>
          <NumberFormat value={runs.failed} />
        </Value>
        <Detail>
          {runs.failed === 0 ? intl.formatMessage(messages.noFailures) : intl.formatMessage(messages.failedShare, { share: intl.formatNumber(failedShare, { style: 'percent', maximumFractionDigits: 1 }) })}
        </Detail>
      </Card>
      <Card icon={ShieldQuestion} label={intl.formatMessage(messages.review)} attention={runs.uncertain > 0}>
        <Value attention={runs.uncertain > 0}>
          <NumberFormat value={runs.uncertain} />
        </Value>
        <Detail>{intl.formatMessage(runs.uncertain > 0 ? messages.reviewDetail : messages.reviewNone)}</Detail>
      </Card>
      <Card icon={Clock} label={intl.formatMessage(messages.timeSaved, { days: timeSaved.windowDays })} wide>
        <Value>{intl.formatMessage(hours > 0 ? messages.hours : messages.minutes, { hours, minutes: timeSaved.minutes })}</Value>
        <Detail>{intl.formatMessage(messages.timeSavedNote, { minutes: timeSaved.minutesPerExecution })}</Detail>
      </Card>
    </ul>
  );
}
