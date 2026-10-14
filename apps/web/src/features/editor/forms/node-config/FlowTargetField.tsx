// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Banner, Checkbox, FormField, Input, Link as UiLink, Select, Skeleton, Switch } from '@orcon/ui';
import { useQuery } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import { ExternalLink } from 'lucide-react';
import { useIntl } from 'react-intl';

import { flowTargetsQueryOptions } from '../../api/queries';
import { expressionSource, isExpression, toExpression, withoutUndefined } from '../../model/node-config';
import { useEditorStore } from '../../store/editor-context';

import { configMessages } from './messages';
import { useFieldGroup, useNodeActions } from './use-node-config';

const NONE = '__none__';

interface CallBlock {
  target: string;
  allowedTargets?: string[];
  version: 'CURRENT' | number;
  mode: 'WAIT' | 'FIRE_AND_FORGET';
  perItem: boolean;
  [member: string]: unknown;
}

/** The `call` block of a node as the form edits it: members that are missing or odd get the format's defaults. */
export function readCall(value: unknown): CallBlock {
  const block = typeof value === 'object' && value !== null && !Array.isArray(value) ? (value as Record<string, unknown>) : {};
  return {
    ...block,
    target: typeof block['target'] === 'string' ? block['target'] : '',
    version: typeof block['version'] === 'number' ? block['version'] : 'CURRENT',
    mode: block['mode'] === 'FIRE_AND_FORGET' ? 'FIRE_AND_FORGET' : 'WAIT',
    perItem: block['perItem'] === true,
  };
}

const choice = 'text-fg bg-surface hover:bg-surface-sunken aria-pressed:bg-surface-sunken min-h-target px-2 text-xs aria-pressed:font-semibold';

/**
 * The widget for the flow a Call sub-flow node runs (SPEC-014 R-ED-SUB-01): a flow of the project or
 * a formula that chooses among allowed flows, the version (current or a fixed number), the mode
 * (wait or fire and forget) and whether it runs per item. "Open sub-flow" goes to the called flow.
 */
export function FlowTargetField({ nodeId, value }: { nodeId: string; value: unknown }) {
  const intl = useIntl();
  const actions = useNodeActions(nodeId);
  const query = useQuery(flowTargetsQueryOptions());
  const ownId = useEditorStore((state) => state.document.id);
  const formulaGroup = useFieldGroup('call-formula');
  const versionGroup = useFieldGroup('call-version');
  const call = readCall(value);
  const formula = isExpression(call.target);
  const flows = query.data ?? [];
  const chosen = flows.find((flow) => flow.id === call.target);
  /** Writes the block with the changes; a member set to `undefined` is removed, not stored. */
  const write = (next: { [Member in keyof CallBlock]?: CallBlock[Member] | undefined }, group?: string) => {
    actions.setBlock('call', withoutUndefined({ ...call, ...next }), group);
  };

  const mode = (id: 'pick' | 'formula') => (
    <button
      key={id}
      type="button"
      aria-pressed={(id === 'formula') === formula}
      onClick={() => {
        if ((id === 'formula') !== formula) write({ target: id === 'formula' ? toExpression('') : '' });
      }}
      className={choice}
    >
      {intl.formatMessage(id === 'pick' ? configMessages.callPick : configMessages.callFormula)}
    </button>
  );

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-col gap-2">
        <div role="group" aria-label={intl.formatMessage(configMessages.callTargetKind)} className="border-control inline-flex self-start overflow-hidden rounded-md border">
          {mode('pick')}
          {mode('formula')}
        </div>
        {formula ? (
          <>
            <FormField label={intl.formatMessage(configMessages.callFormulaLabel)} name="/call/target" required>
              <Input
                className="font-mono text-mono"
                value={expressionSource(call.target)}
                onFocus={formulaGroup.renew}
                onChange={(event) => {
                  write({ target: toExpression(event.target.value) }, formulaGroup.group());
                }}
              />
            </FormField>
            <FormField group label={intl.formatMessage(configMessages.callAllowed)} description={intl.formatMessage(configMessages.callAllowedHint)} name="/call/allowedTargets">
              <div className="flex flex-col gap-1">
                {flows.map((flow) => (
                  <Checkbox
                    key={flow.id}
                    label={flow.name}
                    checked={call.allowedTargets?.includes(flow.id) === true}
                    onCheckedChange={(checked) => {
                      const current = call.allowedTargets ?? [];
                      const next = checked === true ? [...current, flow.id] : current.filter((id) => id !== flow.id);
                      write({ allowedTargets: next.length === 0 ? undefined : next });
                    }}
                  />
                ))}
              </div>
            </FormField>
          </>
        ) : query.isPending ? (
          <Skeleton className="h-control w-full" />
        ) : (
          <FormField label={intl.formatMessage(configMessages.callTarget)} required name="/call/target">
            <Select
              value={call.target === '' ? NONE : call.target}
              placeholder={intl.formatMessage(configMessages.callPlaceholder)}
              className="h-auto min-h-control-target py-1"
              onValueChange={(next) => {
                write({ target: next === NONE ? '' : next });
              }}
              options={[
                ...(call.target === '' ? [{ value: NONE, label: intl.formatMessage(configMessages.callPlaceholder) }] : []),
                ...(call.target !== '' && !chosen ? [{ value: call.target, label: call.target }] : []),
                ...flows.map((flow) => ({ value: flow.id, label: flow.name })),
              ]}
            />
          </FormField>
        )}
        {!formula && call.target !== '' && !chosen && query.isSuccess ? <p className="text-fg-muted text-sm">{intl.formatMessage(configMessages.callUnknown)}</p> : null}
      </div>

      {!formula && call.target === ownId ? <Banner tone="warning" title={intl.formatMessage(configMessages.callSelf)} /> : null}
      {!formula && chosen && !chosen.published && call.version === 'CURRENT' ? <Banner tone="warning" title={intl.formatMessage(configMessages.callUnpublished, { name: chosen.name })} /> : null}

      {!formula && call.target !== '' ? (
        <div className="flex flex-col gap-1">
          <UiLink asChild className="self-start text-sm">
            <Link to="/flows/$flowId" params={{ flowId: call.target }}>
              {intl.formatMessage(configMessages.callOpen)}
              <ExternalLink aria-hidden className="size-icon-16" />
            </Link>
          </UiLink>
          <p className="text-fg-muted text-xs">{intl.formatMessage(configMessages.callOpenHint)}</p>
        </div>
      ) : null}

      <FormField label={intl.formatMessage(configMessages.callVersion)} name="/call/version">
        <Select
          value={call.version === 'CURRENT' ? 'CURRENT' : 'FIXED'}
          onValueChange={(next) => {
            write({ version: next === 'CURRENT' ? 'CURRENT' : 1 });
          }}
          options={[
            { value: 'CURRENT', label: intl.formatMessage(configMessages.callVersionCurrent) },
            { value: 'FIXED', label: intl.formatMessage(configMessages.callVersionFixed) },
          ]}
        />
      </FormField>
      {call.version !== 'CURRENT' ? (
        <FormField label={intl.formatMessage(configMessages.callVersionNumber)} name="/call/versionNumber">
          <Input
            type="number"
            min={1}
            step={1}
            value={call.version}
            onFocus={versionGroup.renew}
            onChange={(event) => {
              const number = Math.max(1, Math.floor(Number(event.target.value)));
              if (Number.isFinite(number)) write({ version: number }, versionGroup.group());
            }}
          />
        </FormField>
      ) : null}
      <FormField label={intl.formatMessage(configMessages.callMode)} name="/call/mode">
        <Select
          value={call.mode}
          onValueChange={(next) => {
            write({ mode: next === 'FIRE_AND_FORGET' ? 'FIRE_AND_FORGET' : 'WAIT' });
          }}
          options={[
            { value: 'WAIT', label: intl.formatMessage(configMessages.callModeWait) },
            { value: 'FIRE_AND_FORGET', label: intl.formatMessage(configMessages.callModeFire) },
          ]}
        />
      </FormField>
      <Switch
        label={intl.formatMessage(configMessages.callPerItem)}
        checked={call.perItem}
        onCheckedChange={(checked) => {
          write({ perItem: checked });
        }}
      />
    </div>
  );
}
