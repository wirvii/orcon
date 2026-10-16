// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { ReactNode } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { cn } from '../../lib/cn';
import { formatDuration } from '../number-format/NumberFormat';
import { statusMap, type StatusFamily, type StatusKey } from '../status-badge/status-map';

const messages = defineMessages({
  notRun: {
    id: 'timeline.notRun',
    defaultMessage: 'Did not run',
    description: 'Shown instead of a duration on a step of a run that never started, because the run stopped or took another path.',
  },
  duration: {
    id: 'timeline.duration',
    defaultMessage: 'Time',
    description: 'Header of the column of the timeline that gives how long each step took.',
  },
  step: {
    id: 'timeline.step',
    defaultMessage: 'Step',
    description: 'Header of the column of the timeline that names each step.',
  },
  barLabel: {
    id: 'timeline.barLabel',
    defaultMessage: '{name} started {start} into the run and took {duration}',
    description: 'Accessible name of the bar of a step in the timeline. {name} is the step, {start} and {duration} are lengths of time.',
  },
});

/** One step of a run, placed on the time axis of the run. */
export interface TimelineStep {
  id: string;
  /** The name of the step as the person knows it. */
  name: string;
  status: StatusKey;
  /** Milliseconds from the start of the run to the start of the step. Absent when the step did not run. */
  startMs?: number | undefined;
  /** How long the step took. Absent while it runs and when it did not run. */
  durationMs?: number | undefined;
  /** A short remark after the name, such as "7 of 12 failed". */
  note?: string | undefined;
  /** The icon of the service of the step, when it has one. */
  icon?: ReactNode;
}

export interface ExecutionTimelineProps {
  steps: readonly TimelineStep[];
  /** The length of the whole run in milliseconds. The bars are drawn as a share of it. */
  totalMs: number;
  /** The step that is open below the timeline. */
  selectedId?: string | undefined;
  /** Called when a person chooses a step. Without it the steps are not buttons. */
  onSelect?: ((id: string) => void) | undefined;
  /** Accessible name of the list of steps. */
  label: string;
  className?: string | undefined;
}

/** The classes of a row of the timeline. */
const rowBase = 'flex min-h-target w-full items-center gap-3 px-3 py-2 text-left max-md:flex-wrap';
const rowSelected = 'bg-surface-hover';
const rowHover = 'hover:bg-surface-hover';

const fill: Record<StatusFamily, string> = {
  neutral: 'fill-status-neutral',
  active: 'fill-status-active',
  success: 'fill-status-success',
  warning: 'fill-status-warning',
  danger: 'fill-status-danger',
  attention: 'fill-status-attention',
};

const iconTone: Record<StatusFamily, string> = {
  neutral: 'text-status-neutral',
  active: 'text-status-active',
  success: 'text-status-success',
  warning: 'text-status-warning',
  danger: 'text-status-danger',
  attention: 'text-status-attention',
};

/** The bar of a step: its start and length as a share of the run, in a 0 to 100 box that stretches to the width. */
function Bar({ startMs, durationMs, totalMs, family, label }: { startMs: number; durationMs: number; totalMs: number; family: StatusFamily; label: string }) {
  const total = totalMs > 0 ? totalMs : 1;
  const x = Math.min(99, Math.max(0, (startMs / total) * 100));
  // A step shorter than a pixel still shows: the bar never gets thinner than one percent.
  const width = Math.min(100 - x, Math.max(1, (durationMs / total) * 100));
  return (
    // eslint-disable-next-line no-restricted-syntax -- a bar drawn from numbers, not a connector logo; its position is a number, and an attribute (unlike a style) needs no inline style
    <svg role="img" aria-label={label} viewBox="0 0 100 8" preserveAspectRatio="none" className="bg-surface-sunken h-2 w-full rounded-sm">
      <rect x={x} y={0} width={width} height={8} className={fill[family]} />
    </svg>
  );
}

/**
 * The steps of one run as rows, each with a bar placed on the time of the run: where the time went and which steps
 * did not run. Status is the icon and the words (read by assistive technology), never the bar's color alone. A step
 * that did not run has no bar and says so in words. With `onSelect` each row is a button that opens that step.
 */
export function ExecutionTimeline({ steps, totalMs, selectedId, onSelect, label, className }: ExecutionTimelineProps) {
  const intl = useIntl();
  return (
    <div className={cn('flex flex-col', className)}>
      <div aria-hidden className="text-fg-muted flex items-center gap-3 px-3 pb-1 text-xs font-medium max-md:hidden">
        <span className="w-1/3 shrink-0">{intl.formatMessage(messages.step)}</span>
        <span className="flex-1" />
        <span className="min-w-16 shrink-0 text-right">{intl.formatMessage(messages.duration)}</span>
      </div>
      <ol aria-label={label} className="border-border flex flex-col rounded-lg border">
        {steps.map((step) => {
          const entry = statusMap[step.status];
          const Icon = entry.icon;
          const ran = step.startMs !== undefined && step.status !== 'NOT_RUN';
          const selected = step.id === selectedId;
          const failed = entry.family === 'danger';
          const content = (
            <>
              <span className="flex min-w-0 items-center gap-2 max-md:basis-full md:w-1/3 md:shrink-0">
                <Icon aria-hidden className={cn('size-icon-16 shrink-0', iconTone[entry.family])} />
                <span className="sr-only">{intl.formatMessage(entry.label)}</span>
                {step.icon ? <span className="shrink-0">{step.icon}</span> : null}
                <span className={cn('truncate', failed && 'font-semibold', !ran && 'text-fg-muted')} data-truncate title={step.name}>
                  {step.name}
                </span>
                {step.note ? <span className="text-fg-muted shrink-0 text-sm">{step.note}</span> : null}
              </span>
              <span className="flex min-w-0 flex-1 items-center">
                {!ran ? <span className="text-fg-muted text-sm">{intl.formatMessage(messages.notRun)}</span> : null}
                {ran ? (
                  <Bar
                    startMs={step.startMs ?? 0}
                    durationMs={step.durationMs ?? 0}
                    totalMs={totalMs}
                    family={entry.family}
                    label={intl.formatMessage(messages.barLabel, {
                      name: step.name,
                      start: formatDuration(step.startMs ?? 0, intl.locale),
                      duration: formatDuration(step.durationMs ?? 0, intl.locale),
                    })}
                  />
                ) : null}
              </span>
              <span className={cn('text-fg-muted min-w-16 shrink-0 text-right text-sm whitespace-nowrap tabular-nums', failed && 'text-status-danger')}>
                {ran && step.durationMs !== undefined ? formatDuration(step.durationMs, intl.locale) : null}
              </span>
            </>
          );
          return (
            <li key={step.id} data-step={step.id} data-status={step.status} className="border-border border-b last:border-b-0">
              {onSelect ? (
                <button
                  type="button"
                  aria-pressed={selected}
                  onClick={() => {
                    onSelect(step.id);
                  }}
                  className={cn(rowBase, selected && rowSelected, rowHover)}
                >
                  {content}
                </button>
              ) : (
                <div className={cn(rowBase, selected && rowSelected)}>{content}</div>
              )}
            </li>
          );
        })}
      </ol>
    </div>
  );
}
