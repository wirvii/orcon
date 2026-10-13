// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Plus } from 'lucide-react';
import type { CSSProperties } from 'react';

import { cn } from '../../lib/cn';

export interface OutputStubProps {
  /** Accessible name: "Add node after <name>, output <output>". */
  label: string;
  /** Vertical position on the right edge of the tile, in px from its top. */
  top?: number | undefined;
  tabbable?: boolean | undefined;
  onAdd?: (() => void) | undefined;
  className?: string | undefined;
}

/**
 * What an output with no connection shows: a 30 px dashed line that ends in a dashed square with a
 * plus. Hovering or focusing the square turns its border and plus to the action color. Activating it
 * opens the node palette to insert a node after this output. The control is the target size (44 px
 * with a coarse pointer) around the 22 px square.
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
        className="nodrag group/stub grid min-h-target min-w-target place-items-center rounded-md bg-transparent p-0 outline-hidden"
      >
        <span
          data-stub-box
          className={cn(
            'bg-canvas-node-bg border-canvas-edge text-fg-muted grid size-canvas-stub-box place-items-center rounded-md border-2 border-dashed',
            'group-hover/stub:border-action-primary group-hover/stub:text-action-primary group-focus-visible/stub:border-action-primary group-focus-visible/stub:text-action-primary',
            'group-focus-visible/stub:outline-focus group-focus-visible/stub:outline-2 group-focus-visible/stub:outline-offset-2',
          )}
        >
          <Plus aria-hidden strokeWidth={3} className="size-3" />
        </span>
      </button>
    </span>
  );
}
