// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Plus } from 'lucide-react';
import type { CSSProperties } from 'react';

import { cn } from '../../lib/cn';

export interface OutputStubProps {
  /** Accessible name: "Add node after <name>, output <output>". */
  label: string;
  /** Vertical position on the right edge of the tile, in px from its top. */
  top?: number;
  tabbable?: boolean;
  onAdd?: () => void;
  className?: string;
}

/**
 * What an output with no connection shows: a 30 px dashed line that ends in a dashed square with a
 * plus. Hovering or focusing the square turns its border and plus to the action color. Activating it
 * opens the node palette to insert a node after this output.
 */
export function OutputStub({ label, top, tabbable = true, onAdd, className }: OutputStubProps) {
  const offset: CSSProperties | undefined = top === undefined ? undefined : { top };
  return (
    <span
      data-output-stub
      // eslint-disable-next-line orcon/no-inline-style -- stub offset computed from the node geometry
      style={offset}
      className={cn('absolute left-full flex -translate-y-1/2 items-center pl-1', className)}
    >
      <span aria-hidden className="border-canvas-edge w-canvas-stub border-t-2 border-dashed" />
      <button
        type="button"
        aria-label={label}
        tabIndex={tabbable ? 0 : -1}
        onClick={onAdd}
        className={cn(
          'nodrag bg-canvas-node-bg border-canvas-edge text-fg-muted grid size-canvas-stub-box place-items-center rounded-md border-2 border-dashed p-0',
          'after:absolute after:-inset-1 pointer-coarse:after:-inset-3',
          'hover:border-action-primary hover:text-action-primary focus-visible:border-action-primary focus-visible:text-action-primary relative',
        )}
      >
        <Plus aria-hidden strokeWidth={3} className="size-3" />
      </button>
    </span>
  );
}
