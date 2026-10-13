// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Handle, Position } from '@xyflow/react';
import type { CSSProperties } from 'react';

import { cn } from '../../lib/cn';
import { PortAddButton } from '../port-add-button/PortAddButton';

export interface NodePortProps {
  side: 'input' | 'output';
  /** Port id inside the node. A connection names its two ports. */
  id: string;
  /** Vertical position on the tile edge, in px from the top of the tile. */
  top?: number | undefined;
  /** Renders the interactive React Flow handle. Without it the port is only drawn (stories). */
  live?: boolean | undefined;
  /** The port can start or end a connection by pointer. False in read-only mode. */
  connectable?: boolean | undefined;
  /** Outputs: the accessible name of the plus button. */
  label?: string | undefined;
  tabbable?: boolean | undefined;
  selected?: boolean | undefined;
  /** Inputs while a connection is dragged: why this input cannot receive it. */
  invalidReason?: string | undefined;
  onAdd?: (() => void) | undefined;
  className?: string | undefined;
}

/**
 * A connection point on a node: a 10 px circle, inputs on the left edge and outputs on the right.
 * Inputs are only a drop target; outputs are the plus button of {@link PortAddButton}. Dragging from a
 * port is a pointer shortcut: the same connection is always available from the keyboard (the Connect
 * form and the flow outline), so an input is hidden from assistive technology.
 */
export function NodePort({ side, id, top, live = false, connectable = true, label = '', tabbable = true, selected = false, invalidReason, onAdd, className }: NodePortProps) {
  if (side === 'output') {
    return <PortAddButton id={id} label={label} top={top} live={live} connectable={connectable} tabbable={tabbable} interactive={connectable} selected={selected} onAdd={onAdd} className={className} />;
  }
  const offset: CSSProperties | undefined = top === undefined ? undefined : { top };
  const look = cn(
    'bg-canvas-node-bg border-canvas-port size-port rounded-full border-2',
    // Extends the drop area beyond the dot for coarse pointers.
    'after:absolute after:-inset-3 pointer-coarse:after:-inset-5',
    // While a connection is dragged, valid inputs grow and take the action color; invalid ones refuse.
    'orcon-handle orcon-handle-in',
    selected && 'border-canvas-selection',
    className,
  );
  if (live) {
    return (
      <Handle
        id={id}
        type="target"
        position={Position.Left}
        isConnectable={connectable}
        aria-hidden
        title={invalidReason}
        // eslint-disable-next-line orcon/no-inline-style -- port offset computed from the node geometry
        style={offset}
        data-port="input"
        data-port-id={id}
        className={cn(look, 'min-h-0 min-w-0')}
      />
    );
  }
  return (
    <span
      aria-hidden
      data-port="input"
      data-port-id={id}
      // eslint-disable-next-line orcon/no-inline-style -- port offset computed from the node geometry
      style={offset}
      className={cn(look, 'absolute left-0 -translate-x-1/2 -translate-y-1/2')}
    />
  );
}
