// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Zap } from 'lucide-react';
import { useIntl } from 'react-intl';

import { ConnectorIcon } from '../components/connector-icon/ConnectorIcon';
import { Tooltip } from '../components/tooltip/Tooltip';
import { cn } from '../lib/cn';

import { canvasMessages } from './messages';
import type { GraphNode } from './model';
import { NodeStatus } from './NodeStatus';
import { nodeAriaLabel, portLabel } from './nodeLabel';
import { Port } from './Port';

export interface NodeCardProps {
  /** An action or a trigger. Control nodes use ControlNode. */
  node: GraphNode;
  selected?: boolean;
  /** Ports are live React Flow handles. True inside the canvas. */
  live?: boolean;
  /** Pointer connections are possible. False in read-only mode. */
  connectable?: boolean;
  /** Give the card its own accessible name. The canvas names the node element instead. */
  labelled?: boolean;
  className?: string;
}

/**
 * An action or trigger node: the connector logo, the node name, one line of what
 * it does, and the execution state as icon + text. Inputs are on the left and
 * outputs on the right; a trigger has no input and a lightning cap at its start.
 */
export function NodeCard({ node, selected = false, live = false, connectable = true, labelled = true, className }: NodeCardProps) {
  const intl = useIntl();
  const trigger = node.kind === 'trigger';
  const labelledInputs = node.inputs.some((port) => portLabel(node.inputs, port));
  const labelledOutputs = node.outputs.some((port) => portLabel(node.outputs, port));

  return (
    <div
      role={labelled ? 'group' : undefined}
      aria-label={labelled ? nodeAriaLabel(intl, node) : undefined}
      data-node-kind={node.kind}
      data-selected={selected || undefined}
      className={cn(
        'bg-surface-raised text-fg border-canvas-node-border relative flex w-node-width flex-col gap-2 rounded-lg border p-3 shadow-sm',
        selected && 'outline-canvas-selection outline-2 outline-offset-0',
        className,
      )}
    >
      {trigger ? (
        <span
          role="img"
          aria-label={intl.formatMessage(canvasMessages.startsFlow)}
          className="bg-surface border-canvas-node-border text-fg absolute top-3 -left-3 inline-flex size-6 items-center justify-center rounded-full border"
        >
          <Zap aria-hidden className="size-icon-16" />
        </span>
      ) : null}
      <div className="flex items-start gap-3">
        {node.connector ? <ConnectorIcon src={node.connector.src} name={node.connector.name} /> : null}
        <div className="min-w-0 flex-1">
          <p className="text-fg text-base font-semibold wrap-anywhere">{node.name}</p>
          {node.summary ? (
            <Tooltip content={node.summary}>
              <p data-truncate className="text-fg-muted truncate text-xs">
                {node.summary}
              </p>
            </Tooltip>
          ) : null}
        </div>
      </div>
      {node.status ? <NodeStatus status={node.status} uncertainCount={node.uncertainCount} /> : null}

      {labelledInputs ? (
        <ul className="flex flex-col gap-1">
          {node.inputs.map((port) => (
            <li key={port.id} className="relative flex h-icon-16 items-center text-xs text-fg-muted">
              <Port side="input" id={port.id} live={live} connectable={connectable} />
              <span className="pl-1">{portLabel(node.inputs, port)}</span>
            </li>
          ))}
        </ul>
      ) : (
        node.inputs.map((port) => <Port key={port.id} side="input" id={port.id} live={live} connectable={connectable} />)
      )}

      {labelledOutputs ? (
        <ul className="flex flex-col gap-1">
          {node.outputs.map((port) => (
            <li key={port.id} className="relative flex h-icon-16 items-center justify-end text-xs text-fg-muted">
              <span className="pr-1">{portLabel(node.outputs, port)}</span>
              <Port side="output" id={port.id} live={live} connectable={connectable} />
            </li>
          ))}
        </ul>
      ) : (
        node.outputs.map((port) => <Port key={port.id} side="output" id={port.id} live={live} connectable={connectable} />)
      )}
    </div>
  );
}
