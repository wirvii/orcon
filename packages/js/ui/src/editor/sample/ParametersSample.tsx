// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { useId, useState, type ReactNode } from 'react';
import { useIntl } from 'react-intl';

import { Banner } from '../../components/banner/Banner';
import { Button } from '../../components/button/Button';
import { FormField } from '../../components/form-field/FormField';
import { Input } from '../../components/input/Input';
import { Select } from '../../components/select/Select';
import { Switch } from '../../components/switch/Switch';
import { Textarea } from '../../components/textarea/Textarea';
import type { GraphNode } from '../../canvas/model';
import { cn } from '../../lib/cn';

import { sampleMessages } from './messages';

/** A field whose value is either fixed or an expression. The switch sits at the right of the label; expressions use mono text. */
function ExpressionField({ label, hint, defaultExpression, children }: { label: string; hint?: string; defaultExpression?: boolean; children: (props: { id: string; expression: boolean }) => ReactNode }) {
  const intl = useIntl();
  const id = useId();
  const [expression, setExpression] = useState(defaultExpression ?? false);
  return (
    <div className="flex flex-col gap-1">
      <div className="flex items-center justify-between gap-2">
        <label htmlFor={id} className="text-sm font-medium">
          {label}
        </label>
        <div role="group" aria-label={intl.formatMessage(sampleMessages.valueKind, { field: label })} className="border-control inline-flex overflow-hidden rounded-md border">
          {[false, true].map((kind) => (
            <button
              key={String(kind)}
              type="button"
              aria-pressed={expression === kind}
              onClick={() => setExpression(kind)}
              className="text-fg bg-surface hover:bg-surface-sunken aria-pressed:bg-surface-sunken min-h-target px-2 text-xs aria-pressed:font-semibold"
            >
              {intl.formatMessage(kind ? sampleMessages.expression : sampleMessages.fixed)}
            </button>
          ))}
        </div>
      </div>
      {children({ id, expression })}
      {hint ? <p className="text-fg-muted text-xs">{hint}</p> : null}
    </div>
  );
}

/** The sample content of the Parameters tab: a credential, an operation, a channel and a message with expressions. */
export function ParametersSample({ node }: { node: GraphNode }) {
  const intl = useIntl();
  if (node.kind === 'logic') {
    return (
      <div className="flex flex-col gap-3">
        <ExpressionField label={intl.formatMessage(sampleMessages.operation)} defaultExpression>
          {({ id, expression }) => <Input id={id} defaultValue="{{ $json.total }} > 0" className={cn(expression && 'font-mono text-mono')} />}
        </ExpressionField>
      </div>
    );
  }
  return (
    <div className="flex flex-col gap-3">
      <FormField label={intl.formatMessage(sampleMessages.credential)}>
        <Select
          defaultValue="team"
          options={[
            { value: 'team', label: intl.formatMessage(sampleMessages.credentialValue) },
            { value: 'other', label: `${intl.formatMessage(sampleMessages.credentialValue)} 2` },
          ]}
        />
      </FormField>
      <FormField label={intl.formatMessage(sampleMessages.operation)}>
        <Select
          defaultValue="send"
          options={[
            { value: 'send', label: intl.formatMessage(sampleMessages.operationSend) },
            { value: 'read', label: intl.formatMessage(sampleMessages.operationRead) },
          ]}
        />
      </FormField>
      <ExpressionField label={intl.formatMessage(sampleMessages.channel)} hint={intl.formatMessage(sampleMessages.channelHint)}>
        {({ id, expression }) => <Input id={id} defaultValue="#orders-alerts" className={cn(expression && 'font-mono text-mono')} />}
      </ExpressionField>
      <ExpressionField label={intl.formatMessage(sampleMessages.message)} defaultExpression>
        {({ id, expression }) => <Textarea id={id} defaultValue={intl.formatMessage(sampleMessages.messageValue, { open: '{{ ', close: ' }}' })} className={cn(expression && 'font-mono text-mono')} />}
      </ExpressionField>
    </div>
  );
}

export function SettingsSample() {
  const intl = useIntl();
  return (
    <div className="flex flex-col gap-3">
      <Switch label={intl.formatMessage(sampleMessages.retryOnFail)} />
      <Switch label={intl.formatMessage(sampleMessages.alwaysOutput)} defaultChecked />
      <FormField label={intl.formatMessage(sampleMessages.notes)}>
        <Textarea />
      </FormField>
    </div>
  );
}

export function DocsSample() {
  const intl = useIntl();
  return (
    <div className="flex flex-col gap-2">
      <h3 className="text-base font-semibold">{intl.formatMessage(sampleMessages.docsTitle)}</h3>
      <p className="text-fg-muted text-base">{intl.formatMessage(sampleMessages.docsText)}</p>
    </div>
  );
}

export function AgentSample() {
  const intl = useIntl();
  return (
    <div className="flex flex-col gap-3">
      <FormField label={intl.formatMessage(sampleMessages.agentModel)}>
        <Select defaultValue="m" options={[{ value: 'm', label: intl.formatMessage(sampleMessages.agentModel) }]} />
      </FormField>
      <FormField label={intl.formatMessage(sampleMessages.agentMemory)}>
        <Input defaultValue="10" />
      </FormField>
    </div>
  );
}

/**
 * The alert that heads the form when the last run left the node failed or uncertain: icon, title,
 * text and actions. An uncertain node never offers "run again" as its primary action: checking
 * what happened comes first.
 */
export function NodeStatusAlert({ node, onAction }: { node: GraphNode; onAction?: (action: 'reconnect' | 'run' | 'mark-sent') => void }) {
  const intl = useIntl();
  if (node.status === 'FAILED' || node.status === 'ERROR_FINAL') {
    return (
      <Banner
        tone="danger"
        title={intl.formatMessage(sampleMessages.alertFailedTitle)}
        action={
          <>
            <Button onClick={() => onAction?.('reconnect')}>{intl.formatMessage(sampleMessages.alertFailedReconnect)}</Button>
            <Button variant="secondary" onClick={() => onAction?.('run')}>
              {intl.formatMessage(sampleMessages.alertFailedRun)}
            </Button>
          </>
        }
      >
        {intl.formatMessage(sampleMessages.alertFailedText)}
      </Banner>
    );
  }
  if (node.status === 'UNCERTAIN') {
    return (
      <Banner
        tone="attention"
        title={intl.formatMessage(sampleMessages.alertUncertainTitle)}
        action={
          <>
            <Button variant="secondary" onClick={() => onAction?.('mark-sent')}>
              {intl.formatMessage(sampleMessages.alertUncertainSent)}
            </Button>
            <Button variant="ghost" onClick={() => onAction?.('run')}>
              {intl.formatMessage(sampleMessages.alertUncertainRun)}
            </Button>
          </>
        }
      >
        {intl.formatMessage(sampleMessages.alertUncertainText)}
      </Banner>
    );
  }
  return null;
}
