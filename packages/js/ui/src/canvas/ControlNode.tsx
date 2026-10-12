// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { GitBranch, Merge, Repeat, Split, Timer, type LucideIcon } from 'lucide-react';
import { useIntl } from 'react-intl';

import { cn } from '../lib/cn';

import type { ControlType, GraphNode } from './model';
import { nodeAriaLabel, portLabel } from './nodeLabel';
import { Port } from './Port';

const icons: Record<ControlType, LucideIcon> = {
  if: GitBranch,
  switch: Split,
  merge: Merge,
  loop: Repeat,
  wait: Timer,
};

export interface ControlNodeProps {
  /** A node of kind "control": if, switch, merge, loop or wait. */
  node: GraphNode;
  selected?: boolean;
  live?: boolean;
  connectable?: boolean;
  labelled?: boolean;
  className?: string;
}

/**
 * A control node (if, switch, merge, loop, wait). It is a pill, smaller than an
 * action card, with its own shape so it is never mistaken for a step that does
 * work. Outputs are labelled ("true", "false", case names).
 */
export function ControlNode({ node, selected = false, live = false, connectable = true, labelled = true, className }: ControlNodeProps) {
  const intl = useIntl();
  const Icon = icons[node.controlType ?? 'if'];
  const labelledInputs = node.inputs.some((port) => portLabel(node.inputs, port));
  return (
    <div
      role={labelled ? 'group' : undefined}
      aria-label={labelled ? nodeAriaLabel(intl, node) : undefined}
      data-node-kind="control"
      data-control-type={node.controlType}
      data-selected={selected || undefined}
      className={cn(
        'bg-surface-raised text-fg border-canvas-node-border relative flex min-h-control-node items-center gap-2 rounded-full border py-1 pl-3 shadow-sm',
        selected && 'outline-canvas-selection outline-2 outline-offset-0',
        className,
      )}
    >
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
      <Icon aria-hidden className="size-icon-16 shrink-0" />
      <span className="text-sm font-semibold whitespace-nowrap">{node.name}</span>
      <ul className="ml-auto flex flex-col gap-1">
        {node.outputs.map((port) => (
          <li key={port.id} className="relative flex h-icon-16 items-center justify-end pr-3 text-xs text-fg-muted">
            <span className="pl-2">{portLabel(node.outputs, port)}</span>
            <Port side="output" id={port.id} live={live} connectable={connectable} />
          </li>
        ))}
      </ul>
    </div>
  );
}
