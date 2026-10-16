// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Spinner, StatusBadge } from '@orcon/ui';
import { CircleCheck, CircleX, ChevronDown, ChevronUp, Clock, type LucideIcon } from 'lucide-react';
import type { ReactNode } from 'react';
import { defineMessages, useIntl, type IntlShape } from 'react-intl';

const messages = defineMessages({
  title: { id: 'editor.logs.title', defaultMessage: 'Logs', description: 'Title of the panel at the bottom of the editor that shows what the last run of the flow did.' },
  toggle: { id: 'editor.logs.toggle', defaultMessage: 'Show or hide the logs', description: 'Accessible name of the button that opens or closes the logs panel.' },
  steps: { id: 'editor.logs.steps', defaultMessage: '{count, plural, one {# step} other {# steps}}', description: 'How many steps the last run went through.' },
  tokens: { id: 'editor.logs.tokens', defaultMessage: '{count, number} tokens', description: 'How many tokens the AI steps of the last run used. Only shown when the run reports them.' },
  ms: { id: 'editor.logs.ms', defaultMessage: '{value, number} ms', description: 'A duration of less than a second, in milliseconds.' },
  seconds: { id: 'editor.logs.seconds', defaultMessage: '{value, number} s', description: 'A duration of a second or more, in seconds with one decimal.' },
  loading: { id: 'editor.logs.loading', defaultMessage: 'Reading the last run…', description: 'Shown in the logs panel while the last run is being read.' },
  empty: { id: 'editor.logs.empty', defaultMessage: 'This flow has not run yet. Its runs show here.', description: 'Shown in the logs panel when the flow has no runs.' },
  unavailable: {
    id: 'editor.logs.unavailable',
    defaultMessage: 'The runs of a flow are not available yet on this server.',
    description: 'Shown in the logs panel when the server does not answer the list of runs yet. It is not an error.',
  },
  error: { id: 'editor.logs.error', defaultMessage: 'The last run could not be read.', description: 'Shown in the logs panel when reading the last run failed.' },
  stepsList: { id: 'editor.logs.stepsList', defaultMessage: 'Steps of the run', description: 'Accessible name of the list of steps the last run went through.' },
  output: { id: 'editor.logs.output', defaultMessage: '{name}: output', description: 'Title of the output of a step in the logs panel. {name} is the step name.' },
  noOutput: { id: 'editor.logs.noOutput', defaultMessage: 'This step has no output to show.', description: 'Shown when the selected step of the run produced no data, or the person may not read it.' },
  pick: { id: 'editor.logs.pick', defaultMessage: 'Choose a step to see its output.', description: 'Shown in the logs panel before a step is chosen.' },
  inner: { id: 'editor.logs.inner', defaultMessage: 'Inside the agent', description: 'Accessible name of the list of steps an agent step went through inside (memory, tools, model calls).' },
});

export type LogsState = 'loading' | 'empty' | 'unavailable' | 'error' | 'ready';

export interface LogStep {
  id: string;
  name: string;
  /** `SUCCEEDED`, `FAILED`... as the node run reports it. */
  ok: boolean | undefined;
  durationMs?: number | undefined;
  /** The steps inside an agent step, when the run records them. */
  inner?: readonly { name: string; durationMs?: number | undefined }[] | undefined;
}

export interface LogsRun {
  status: Parameters<typeof StatusBadge>[0]['status'];
  durationMs?: number | undefined;
  /** Already formatted: "3 min ago". */
  when?: string | undefined;
  /** Total tokens of the AI steps. Absent when the run does not report them: the panel then leaves them out, it never writes 0. */
  tokens?: number | undefined;
}

export interface LogsPanelProps {
  state: LogsState;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  run?: LogsRun | undefined;
  steps?: readonly LogStep[] | undefined;
  selectedId?: string | undefined;
  onSelect?: ((id: string) => void) | undefined;
  /** The output of the selected step, as the run recorded it. */
  output?: unknown;
  /** The test chat, shown beside the output while it is open. */
  chat?: ReactNode;
  /** The link to the runs of the flow, shown at the right of the bar. */
  previousRuns?: ReactNode;
}

/** A duration in the person's words: "288 ms", "2.4 s". */
export function formatMs(intl: IntlShape, milliseconds: number): string {
  return milliseconds < 1000 ? intl.formatMessage(messages.ms, { value: Math.round(milliseconds) }) : intl.formatMessage(messages.seconds, { value: Math.round(milliseconds / 100) / 10 });
}

const stepIcon = (ok: boolean | undefined): LucideIcon => (ok === undefined ? Clock : ok ? CircleCheck : CircleX);

/**
 * The logs at the bottom of the editor (SPEC-019 R-UX-EDT-06): a bar that says how the last run went and, open, the
 * steps with their duration, the output of the chosen step and the test chat. What the run does not report is left
 * out: a token count the run does not carry is not written as zero.
 */
export function LogsPanel({ state, open, onOpenChange, run, steps = [], selectedId, onSelect, output, chat, previousRuns }: LogsPanelProps) {
  const intl = useIntl();
  const Chevron = open ? ChevronDown : ChevronUp;
  const selected = steps.find((step) => step.id === selectedId);
  const summary =
    state === 'ready' && run
      ? [
          intl.formatMessage(messages.steps, { count: steps.length }),
          run.durationMs === undefined ? undefined : formatMs(intl, run.durationMs),
          run.tokens === undefined ? undefined : intl.formatMessage(messages.tokens, { count: run.tokens }),
          run.when,
        ]
          .filter((piece): piece is string => piece !== undefined)
          .join(' · ')
      : undefined;
  const note = state === 'loading' ? messages.loading : state === 'empty' ? messages.empty : state === 'unavailable' ? messages.unavailable : state === 'error' ? messages.error : undefined;

  return (
    <section
      aria-label={intl.formatMessage(messages.title)}
      data-logs-panel
      data-open={open}
      className={`border-border bg-surface flex min-h-0 flex-col border-t ${open ? 'h-1/3' : ''}`}
    >
      <div className="flex min-h-12 flex-wrap items-center gap-x-3 gap-y-1 px-4 py-1">
        <button
          type="button"
          aria-expanded={open}
          aria-label={intl.formatMessage(messages.toggle)}
          onClick={() => {
            onOpenChange(!open);
          }}
          className="hover:bg-surface-sunken inline-flex min-h-target min-w-target items-center justify-center rounded-md"
        >
          <Chevron aria-hidden className="size-icon-16" />
        </button>
        <h2 className="text-base font-semibold">{intl.formatMessage(messages.title)}</h2>
        {state === 'loading' ? <Spinner size={16} decorative /> : null}
        {run ? <StatusBadge status={run.status} className="shrink-0 whitespace-nowrap" /> : null}
        {summary ? <span className="text-fg-muted text-sm">{summary}</span> : note ? <span className="text-fg-muted text-sm">{intl.formatMessage(note)}</span> : null}
        <span className="ms-auto text-sm">{previousRuns}</span>
      </div>
      {open && state !== 'ready' && chat ? <div className="border-border flex min-h-0 flex-1 border-t">{chat}</div> : null}
      {open && state === 'ready' ? (
        <div className="border-border flex min-h-0 flex-1 flex-col border-t md:flex-row">
          <ol aria-label={intl.formatMessage(messages.stepsList)} className="border-border min-h-0 overflow-y-auto border-e p-2 md:w-canvas-palette-w md:shrink-0">
            {steps.map((step) => {
              const Icon = stepIcon(step.ok);
              return (
                <li key={step.id}>
                  <button
                    type="button"
                    aria-current={step.id === selectedId ? 'true' : undefined}
                    onClick={() => onSelect?.(step.id)}
                    className="hover:bg-surface-sunken aria-[current=true]:bg-surface-sunken flex min-h-target w-full items-center gap-2 rounded-md px-2 text-start text-sm"
                  >
                    <Icon aria-hidden className={`size-icon-16 shrink-0 ${step.ok === false ? 'text-status-danger' : step.ok ? 'text-status-success' : 'text-fg-muted'}`} />
                    <span className="min-w-0 flex-1 truncate">{step.name}</span>
                    {step.durationMs === undefined ? null : <span className="text-fg-muted text-xs">{formatMs(intl, step.durationMs)}</span>}
                  </button>
                  {step.inner && step.inner.length > 0 ? (
                    <ul aria-label={intl.formatMessage(messages.inner)} className="ms-6 flex flex-col">
                      {step.inner.map((inner) => (
                        <li key={inner.name} className="text-fg-muted flex min-h-8 items-center gap-2 px-2 text-sm">
                          <span className="min-w-0 flex-1 truncate">{inner.name}</span>
                          {inner.durationMs === undefined ? null : <span className="text-xs">{formatMs(intl, inner.durationMs)}</span>}
                        </li>
                      ))}
                    </ul>
                  ) : null}
                </li>
              );
            })}
          </ol>
          <div className="min-h-0 min-w-0 flex-1 overflow-auto p-4">
            {selected ? (
              <>
                <h3 className="text-sm font-semibold">{intl.formatMessage(messages.output, { name: selected.name })}</h3>
                {output === undefined ? (
                  <p className="text-fg-muted mt-2 text-sm">{intl.formatMessage(messages.noOutput)}</p>
                ) : (
                  <pre className="text-fg mt-2 font-mono text-sm break-words whitespace-pre-wrap">{JSON.stringify(output, null, 2)}</pre>
                )}
              </>
            ) : (
              <p className="text-fg-muted text-sm">{intl.formatMessage(messages.pick)}</p>
            )}
          </div>
          {chat ? <div className="border-border flex min-h-0 border-t md:w-canvas-palette-w md:shrink-0 md:border-s md:border-t-0">{chat}</div> : null}
        </div>
      ) : null}
    </section>
  );
}
