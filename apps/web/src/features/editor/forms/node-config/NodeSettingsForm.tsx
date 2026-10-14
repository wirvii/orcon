// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Banner, FormField, Input, Select, Switch, Textarea } from '@orcon/ui';
import { useState, type ReactNode } from 'react';
import { useIntl } from 'react-intl';

import { formatDuration, parseDuration } from '../../model/duration';
import { MAX_NODE_NOTE } from '../../model/flow-document';
import type { FlowNode } from '../../model/flow-document';
import { withoutUndefined } from '../../model/node-config';
import { effectOf, type NodeManifest } from '../../model/node-manifests';

import { configMessages } from './messages';
import { useEditorStore } from '../../store/editor-context';

import { useFieldGroup, useNodeActions } from './use-node-config';

type Block = Record<string, unknown>;

const block = (value: unknown): Block => (typeof value === 'object' && value !== null && !Array.isArray(value) ? { ...(value as Block) } : {});
const seconds = (value: unknown): number | undefined => (typeof value === 'string' ? parseDuration(value) : undefined);
const format = (value: number | undefined): string => (value === undefined ? '' : String(value));

interface NumberSettingProps {
  label: string;
  hint?: string | undefined;
  name: string;
  value: number | undefined;
  /** The smallest value that is valid. */
  min: number;
  integer: boolean;
  onChange: (value: number | undefined, group: string) => void;
}

/** The number a text holds, `undefined` for an empty text and NaN for text that is not a number. */
function parse(text: string): number | undefined {
  const trimmed = text.trim();
  return trimmed === '' ? undefined : Number(trimmed.replace(',', '.'));
}

/**
 * A number field of the Settings tab. It keeps the text the person typed, so "1." and "" survive
 * while typing, and takes the value of the draft again when the draft changes behind it (undo).
 */
function NumberSetting({ label, hint, name, value, min, integer, onChange }: NumberSettingProps) {
  const intl = useIntl();
  const group = useFieldGroup(name);
  const [state, setState] = useState({ text: format(value), seen: value });
  const [touched, setTouched] = useState(false);
  if (state.seen !== value) {
    setState({ text: parse(state.text) === value ? state.text : format(value), seen: value });
  }
  const parsed = parse(state.text);
  const bad = parsed !== undefined && (Number.isNaN(parsed) || parsed < min || (integer && !Number.isInteger(parsed)));
  return (
    <FormField label={label} description={hint} name={`/${name}`} error={touched && bad ? intl.formatMessage(integer ? configMessages.wholeNumber : configMessages.atLeastOne) : undefined}>
      <Input
        type="text"
        inputMode={integer ? 'numeric' : 'decimal'}
        value={state.text}
        onFocus={group.renew}
        onBlur={() => {
          setTouched(true);
        }}
        onChange={(event) => {
          const text = event.target.value;
          const next = parse(text);
          setState({ text, seen: state.seen });
          if (next === undefined) onChange(undefined, group.group());
          else if (!Number.isNaN(next) && next >= min && (!integer || Number.isInteger(next))) onChange(next, group.group());
        }}
      />
    </FormField>
  );
}

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="flex flex-col gap-3">
      <h3 className="text-base font-semibold">{title}</h3>
      {children}
    </section>
  );
}

const NOTIFY = ['DEFAULT', 'ON', 'OFF'] as const;
const ON_ERROR = ['STOP_EXECUTION', 'CONTINUE_WITH_ERROR', 'ERROR_OUTPUT'] as const;

/** Elements that wait for a person or an outside event: their notifications are a setting (orcon-flow-1.md `attention`). */
const WAITS: ReadonlySet<string> = new Set(['receiveTask', 'userTask', 'intermediateCatchEvent']);

/**
 * The Settings tab of a node (SPEC-014 R-ED-CFG-04): retries, time limits, what happens when the
 * step fails, how many items it handles at once and whether people are told when it needs a decision.
 * Each field writes one behaviour block of the node. The defaults the text explains come from the
 * kind of step (ADR-0011 §2). Notes on a step and marks for sensitive values have no place in the
 * flow format yet, so they are not offered.
 */
export function NodeSettingsForm({ node, manifest }: { node: FlowNode; manifest: NodeManifest | undefined }) {
  const intl = useIntl();
  const actions = useNodeActions(node.id);
  const effect = manifest ? effectOf(manifest, node.parameters) : 'pure';
  const retry = block(node['retry']);
  const timeouts = block(node['timeouts']);
  const attempts = typeof retry['maxAttempts'] === 'number' ? retry['maxAttempts'] : undefined;
  const onError = ON_ERROR.find((option) => option === node['onError']) ?? 'STOP_EXECUTION';
  const concurrency = node['concurrency'];
  const notify = NOTIFY.find((option) => option === block(node['attention'])['notify']) ?? 'DEFAULT';
  const notifyApplies = effect === 'write' || WAITS.has(node.element);
  const canSilence = effect !== 'write';

  const writeRetry = (patch: Block, group: string) => {
    const next = withoutUndefined({ ...retry, ...patch });
    actions.setBlock('retry', typeof next['maxAttempts'] === 'number' ? next : undefined, group);
  };
  const writeTimeout = (key: 'attempt' | 'node', value: number | undefined, group: string) => {
    const next = withoutUndefined({ ...timeouts, [key]: value === undefined ? undefined : formatDuration(value) });
    actions.setBlock('timeouts', Object.keys(next).length === 0 ? undefined : next, group);
  };

  const retryDefault = effect === 'write' ? configMessages.retryDefaultWrite : effect === 'read' ? configMessages.retryDefaultRead : configMessages.retryDefaultPure;

  return (
    <div className="flex flex-col gap-5">
      <Section title={intl.formatMessage(configMessages.settingsRetry)}>
        <p className="text-fg-muted text-sm">{intl.formatMessage(retryDefault)}</p>
        <NumberSetting
          name="retry/maxAttempts"
          label={intl.formatMessage(configMessages.retryAttempts)}
          hint={intl.formatMessage(configMessages.retryAttemptsHint)}
          value={attempts}
          min={1}
          integer
          onChange={(value, group) => {
            writeRetry({ maxAttempts: value }, group);
          }}
        />
        {attempts !== undefined && attempts > 1 ? (
          <>
            <NumberSetting
              name="retry/initialInterval"
              label={intl.formatMessage(configMessages.retryInitial)}
              value={seconds(retry['initialInterval'])}
              min={1}
              integer
              onChange={(value, group) => {
                writeRetry({ initialInterval: value === undefined ? undefined : formatDuration(value) }, group);
              }}
            />
            <NumberSetting
              name="retry/factor"
              label={intl.formatMessage(configMessages.retryFactor)}
              value={typeof retry['factor'] === 'number' ? retry['factor'] : undefined}
              min={1}
              integer={false}
              onChange={(value, group) => {
                writeRetry({ factor: value }, group);
              }}
            />
            <NumberSetting
              name="retry/maxInterval"
              label={intl.formatMessage(configMessages.retryMax)}
              value={seconds(retry['maxInterval'])}
              min={1}
              integer
              onChange={(value, group) => {
                writeRetry({ maxInterval: value === undefined ? undefined : formatDuration(value) }, group);
              }}
            />
            <Switch
              label={intl.formatMessage(configMessages.retryAfter)}
              checked={retry['respectRetryAfter'] === true}
              onCheckedChange={(checked) => {
                writeRetry({ respectRetryAfter: checked ? true : undefined }, 'retry-after');
              }}
            />
          </>
        ) : null}
      </Section>

      <Section title={intl.formatMessage(configMessages.settingsTimeouts)}>
        <NumberSetting
          name="timeouts/attempt"
          label={intl.formatMessage(configMessages.timeoutAttempt)}
          hint={intl.formatMessage(configMessages.timeoutHint)}
          value={seconds(timeouts['attempt'])}
          min={1}
          integer
          onChange={(value, group) => {
            writeTimeout('attempt', value, group);
          }}
        />
        <NumberSetting
          name="timeouts/node"
          label={intl.formatMessage(configMessages.timeoutNode)}
          hint={intl.formatMessage(configMessages.timeoutHint)}
          value={seconds(timeouts['node'])}
          min={1}
          integer
          onChange={(value, group) => {
            writeTimeout('node', value, group);
          }}
        />
      </Section>

      <Section title={intl.formatMessage(configMessages.settingsErrors)}>
        <FormField label={intl.formatMessage(configMessages.onError)} name="/onError">
          <Select
            value={onError}
            onValueChange={(next) => {
              actions.setBlock('onError', next === 'STOP_EXECUTION' ? undefined : next);
            }}
            options={[
              { value: 'STOP_EXECUTION', label: intl.formatMessage(configMessages.onErrorStop) },
              { value: 'CONTINUE_WITH_ERROR', label: intl.formatMessage(configMessages.onErrorContinue) },
              { value: 'ERROR_OUTPUT', label: intl.formatMessage(configMessages.onErrorOutput) },
            ]}
          />
        </FormField>
        {onError === 'ERROR_OUTPUT' ? <p className="text-fg-muted text-sm">{intl.formatMessage(configMessages.onErrorOutputHint)}</p> : null}
      </Section>

      <Section title={intl.formatMessage(configMessages.settingsConcurrency)}>
        <FormField label={intl.formatMessage(configMessages.concurrency)} description={intl.formatMessage(configMessages.concurrencyHint)} name="/concurrency">
          <ConcurrencyInput value={concurrency} onChange={(value, group) => { actions.setBlock('concurrency', value, group); }} />
        </FormField>
      </Section>

      {notifyApplies ? (
        <Section title={intl.formatMessage(configMessages.settingsAttention)}>
          <FormField label={intl.formatMessage(configMessages.attention)} name="/attention/notify">
            <Select
              value={canSilence || notify !== 'OFF' ? notify : 'DEFAULT'}
              onValueChange={(next) => {
                actions.setBlock('attention', next === 'DEFAULT' ? undefined : { ...block(node['attention']), notify: next });
              }}
              options={[
                { value: 'DEFAULT', label: intl.formatMessage(configMessages.attentionDefault) },
                { value: 'ON', label: intl.formatMessage(configMessages.attentionOn) },
                ...(canSilence ? [{ value: 'OFF', label: intl.formatMessage(configMessages.attentionOff) }] : []),
              ]}
            />
          </FormField>
          {!canSilence ? <p className="text-fg-muted text-sm">{intl.formatMessage(configMessages.attentionOffRefused)}</p> : null}
        </Section>
      ) : null}

      <Section title={intl.formatMessage(configMessages.settingsNotes)}>
        <NoteField nodeId={node.id} />
      </Section>

      <Banner tone="info" title={intl.formatMessage(configMessages.settingsLater)} />
    </div>
  );
}

/** The words that mean no limit (`concurrency: ALL`), in the two languages of the interface. */
const NO_LIMIT: ReadonlySet<string> = new Set(['all', 'todos']);

/** A whole number from 1, or the word "all" for no limit (`concurrency` is a number or `ALL`). */
function ConcurrencyInput({ value, onChange }: { value: unknown; onChange: (value: number | 'ALL' | undefined, group: string) => void }) {
  const group = useFieldGroup('concurrency');
  const shown = value === 'ALL' ? 'all' : typeof value === 'number' ? String(value) : '';
  const [text, setText] = useState(shown);
  const [seen, setSeen] = useState(value);
  if (seen !== value) {
    setSeen(value);
    const current = NO_LIMIT.has(text.trim().toLowerCase()) ? 'ALL' : parse(text);
    if (current !== value && !(current === undefined && value === undefined)) setText(shown);
  }
  return (
    <Input
      type="text"
      inputMode="numeric"
      value={text}
      onFocus={group.renew}
      onChange={(event) => {
        const next = event.target.value;
        setText(next);
        const trimmed = next.trim().toLowerCase();
        if (trimmed === '') onChange(undefined, group.group());
        else if (NO_LIMIT.has(trimmed)) onChange('ALL', group.group());
        else {
          const number = Number(trimmed);
          if (Number.isInteger(number) && number >= 1) onChange(number, group.group());
        }
      }}
    />
  );
}

/** The note of a node: plain text kept with the draft, outside the part of the flow that runs. */
function NoteField({ nodeId }: { nodeId: string }) {
  const intl = useIntl();
  const actions = useNodeActions(nodeId);
  const group = useFieldGroup('note');
  const note = useEditorStore((state) => state.document.editor.nodes?.[nodeId]?.note) ?? '';
  return (
    <FormField label={intl.formatMessage(configMessages.note)} description={intl.formatMessage(configMessages.noteHint, { max: MAX_NODE_NOTE })} name="/editor/note">
      <Textarea
        rows={4}
        maxLength={MAX_NODE_NOTE}
        value={note}
        onFocus={group.renew}
        onChange={(event) => {
          actions.setNote(event.target.value === '' ? undefined : event.target.value, group.group());
        }}
      />
    </FormField>
  );
}
