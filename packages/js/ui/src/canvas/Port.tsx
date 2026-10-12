// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Handle, Position } from '@xyflow/react';

import { cn } from '../lib/cn';

export interface PortProps {
  side: 'input' | 'output';
  /** Port id inside the node. A connection names its two ports. */
  id: string;
  /** Renders the interactive React Flow handle. Without it the port is only drawn (stories, outline). */
  live?: boolean;
  /** The port can start or end a connection by pointer. False in read-only mode. */
  connectable?: boolean;
  selected?: boolean;
  className?: string;
}

/**
 * A connection point on a node: 10 px, inputs on the left and outputs on the
 * right. Dragging from a port is a pointer shortcut; the same connection is
 * always available from the keyboard (Connect form and the flow outline), so the
 * port itself is hidden from assistive technology.
 */
export function Port({ side, id, live = false, connectable = true, selected = false, className }: PortProps) {
  const look = cn(
    'bg-canvas-port border-surface size-port rounded-full border-2',
    // Extends the grab area beyond the visible dot for coarse pointers.
    'after:absolute after:-inset-3',
    selected && 'bg-canvas-selection',
    className,
  );
  if (live) {
    return (
      <Handle
        id={id}
        type={side === 'input' ? 'target' : 'source'}
        position={side === 'input' ? Position.Left : Position.Right}
        isConnectable={connectable}
        aria-hidden
        className={cn(look, 'min-h-0 min-w-0')}
      />
    );
  }
  return (
    <span
      aria-hidden
      data-port={side}
      data-port-id={id}
      className={cn(look, 'absolute top-1/2 -translate-y-1/2', side === 'input' ? 'left-0 -translate-x-1/2' : 'right-0 translate-x-1/2')}
    />
  );
}
