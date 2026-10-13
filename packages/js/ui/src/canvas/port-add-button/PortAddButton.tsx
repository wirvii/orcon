// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Handle, Position } from '@xyflow/react';
import { Plus } from 'lucide-react';
import type { CSSProperties, KeyboardEvent } from 'react';

import { cn } from '../../lib/cn';

export interface PortAddButtonProps {
  /** Port id inside the node. */
  id: string;
  /** Accessible name: "Add node after <name>, output <output>". */
  label: string;
  /** Vertical position on the right edge of the tile, in px from its top. */
  top?: number;
  /** Renders the interactive React Flow handle. Without it the button is only drawn (stories). */
  live?: boolean;
  /** The port can start a connection by pointer. False in read-only mode. */
  connectable?: boolean;
  /** Reachable with Tab. Only the node that holds the roving tab stop sets it. */
  tabbable?: boolean;
  /** The port can be used. False in read-only mode: it is only a dot. */
  interactive?: boolean;
  selected?: boolean;
  /** Activation by click, Enter or Space: opens the node palette to insert a node after this output. */
  onAdd?: () => void;
  className?: string;
}

const dot = 'bg-canvas-node-bg border-canvas-port text-action-primary-fg z-20 grid size-port place-items-center rounded-full border-2 p-0';

const interactiveLook = [
  // The visible dot is 10 px; the hit area reaches 34 px, and 50 px under a coarse pointer.
  'after:absolute after:-inset-3 pointer-coarse:after:-inset-5',
  'hover:size-canvas-port-hover hover:bg-action-primary hover:border-action-primary',
  'focus-visible:size-canvas-port-hover focus-visible:bg-action-primary focus-visible:border-action-primary',
  'orcon-handle hover:[&>svg]:opacity-100 focus-visible:[&>svg]:opacity-100',
].join(' ');

/**
 * The output port of a node. At rest it is a 10 px dot; on hover or focus it grows to 24 px, fills
 * with the action color and shows a plus. Activating it opens the node palette to insert a node
 * after this output; dragging from it starts a connection.
 */
export function PortAddButton({ id, label, top, live = false, connectable = true, tabbable = true, interactive = true, selected = false, onAdd, className }: PortAddButtonProps) {
  const position: CSSProperties | undefined = top === undefined ? undefined : { top };
  const onKeyDown = (event: KeyboardEvent<HTMLElement>) => {
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      event.stopPropagation();
      onAdd?.();
    }
  };
  const common = {
    role: interactive ? 'button' : undefined,
    tabIndex: interactive ? (tabbable ? 0 : -1) : undefined,
    'aria-label': interactive ? label : undefined,
    'aria-hidden': interactive ? undefined : true,
    'data-port': 'output',
    'data-port-id': id,
    onKeyDown: interactive ? onKeyDown : undefined,
    className: cn(dot, interactive ? interactiveLook : 'pointer-events-none', selected && 'border-canvas-selection', className),
    children: interactive ? <Plus aria-hidden strokeWidth={3} className="size-3 opacity-0 transition-opacity" /> : null,
  };
  if (live) {
    return (
      <Handle
        id={id}
        type="source"
        position={Position.Right}
        isConnectable={connectable}
        // The port sits at a runtime offset on the tile edge (one of several outputs).
        // eslint-disable-next-line orcon/no-inline-style -- port offset computed from the node geometry
        style={position}
        onClick={(event) => {
          if (!interactive) return;
          event.stopPropagation();
          onAdd?.();
        }}
        {...common}
        className={cn(common.className, 'min-h-0 min-w-0')}
      />
    );
  }
  return (
    <span
      // eslint-disable-next-line orcon/no-inline-style -- port offset computed from the node geometry
      style={position}
      {...common}
      onClick={interactive ? onAdd : undefined}
      className={cn(common.className, 'absolute right-0 translate-x-1/2 -translate-y-1/2')}
    />
  );
}
