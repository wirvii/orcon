// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { ShieldQuestion } from 'lucide-react';
import { useId, useState } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { cn } from '../../lib/cn';
import { Button } from '../button/Button';
import { Checkbox } from '../checkbox/Checkbox';
import { statusMessages } from '../status-badge/status-map';

const messages = defineMessages({
  title: {
    id: 'uncertain.title',
    defaultMessage: 'Review before continuing',
    description: 'Heading of the panel where a person decides what to do with an operation whose result is not known.',
  },
  manualCheck: {
    id: 'uncertain.manualCheck',
    defaultMessage: 'How to check by hand',
    description: 'Label before the connector instructions that explain how to find out whether the operation was applied.',
  },
  actions: {
    id: 'uncertain.actions',
    defaultMessage: 'What you can do',
    description: 'Label before the list of resolutions the person can choose from.',
  },
  noActions: {
    id: 'uncertain.noActions',
    defaultMessage: 'This connector offers no resolution. You can cancel the execution.',
    description: 'Shown when the connector sent no resolutions for an uncertain operation.',
  },
  riskAck: {
    id: 'uncertain.riskAck',
    defaultMessage: 'I understand the risk: {risk}',
    description: 'Checkbox that a person ticks before a risky resolution is enabled. {risk} says what could go wrong.',
  },
  cancel: {
    id: 'uncertain.cancel',
    defaultMessage: 'Cancel the execution',
    description: 'Button that cancels the whole execution instead of resolving the uncertain operation.',
  },
});

export interface UncertainResolutionAction {
  id: string;
  /** What the action does, in the words of the connector. */
  label: string;
  /**
   * `repeat` runs the operation again. A repeat is never the primary action of an
   * uncertain item: the operation may already have happened.
   */
  kind: 'repeat' | 'mark-applied' | 'mark-not-applied' | 'other';
  /** A lasting effect is possible: the action stays disabled until the person ticks the acknowledgment. */
  requiresRiskAck?: boolean | undefined;
  /** The risk the acknowledgment states. */
  riskText?: string | undefined;
}

export interface UncertainResolutionPanelProps {
  /** Node name, for the heading. */
  nodeName: string;
  /** The connector's instructions for checking by hand. Shown as plain text, exactly as received. */
  manualCheck?: string | undefined;
  /** The supervised resolutions the attention request offers, received as data. The panel invents none. */
  actions: readonly UncertainResolutionAction[];
  onResolve?: ((actionId: string) => void) | undefined;
  onCancel?: (() => void) | undefined;
  className?: string | undefined;
}

function ResolutionRow({ action, onResolve }: { action: UncertainResolutionAction; onResolve?: ((id: string) => void) | undefined }) {
  const intl = useIntl();
  const [acknowledged, setAcknowledged] = useState(false);
  const locked = action.requiresRiskAck === true && !acknowledged;
  return (
    <li className="flex flex-col gap-2" data-resolution={action.id}>
      {action.requiresRiskAck ? (
        <Checkbox
          checked={acknowledged}
          onCheckedChange={(next) => setAcknowledged(next === true)}
          label={intl.formatMessage(messages.riskAck, { risk: action.riskText ?? '' })}
        />
      ) : null}
      {/* Nothing here is a primary action. Even a safe resolution is secondary, and a repeat never is. */}
      <Button variant="secondary" disabled={locked} onClick={() => onResolve?.(action.id)} className="self-start whitespace-normal text-left h-auto py-2">
        {action.label}
      </Button>
    </li>
  );
}

/**
 * Where a person decides what to do with an UNCERTAIN item: the external system
 * may or may not have applied the operation. It shows the connector's manual
 * check text as plain text, the supervised resolutions it received as data, and a
 * way to cancel. It never names the situation a failure or a success, never makes
 * "repeat" the primary action, and keeps risky actions disabled until the person
 * acknowledges the risk.
 */
export function UncertainResolutionPanel({ nodeName, manualCheck, actions, onResolve, onCancel, className }: UncertainResolutionPanelProps) {
  const intl = useIntl();
  const titleId = useId();
  return (
    <section
      aria-labelledby={titleId}
      data-uncertain-panel
      className={cn('bg-surface text-fg border-status-attention flex flex-col gap-3 rounded-lg border-2 p-4', className)}
    >
      <div className="text-status-attention flex items-start gap-2">
        <ShieldQuestion aria-hidden className="size-icon-20 shrink-0" />
        <div className="min-w-0">
          <h2 id={titleId} className="text-md font-semibold wrap-anywhere">
            {intl.formatMessage(messages.title)}
          </h2>
          <p className="text-sm font-medium wrap-anywhere">{`${nodeName} · ${intl.formatMessage(statusMessages.uncertain)}`}</p>
        </div>
      </div>
      <p className="wrap-anywhere">{intl.formatMessage(statusMessages.uncertainDescription)}</p>
      {manualCheck ? (
        <div className="flex flex-col gap-1">
          <p className="text-sm font-semibold wrap-anywhere">{intl.formatMessage(messages.manualCheck)}</p>
          <p className="wrap-anywhere whitespace-pre-wrap" data-manual-check>
            {manualCheck}
          </p>
        </div>
      ) : null}
      <div className="flex flex-col gap-2">
        <p className="text-sm font-semibold wrap-anywhere">{intl.formatMessage(messages.actions)}</p>
        {actions.length === 0 ? (
          <p className="wrap-anywhere">{intl.formatMessage(messages.noActions)}</p>
        ) : (
          <ul className="flex flex-col gap-3">
            {actions.map((action) => (
              <ResolutionRow key={action.id} action={action} onResolve={onResolve} />
            ))}
          </ul>
        )}
      </div>
      <Button variant="ghost" onClick={onCancel} className="self-start whitespace-normal text-left h-auto py-2">
        {intl.formatMessage(messages.cancel)}
      </Button>
    </section>
  );
}
