// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { useRef, type KeyboardEvent, type PointerEvent } from 'react';

import { cn } from '../../lib/cn';

export interface PaneSplitterProps {
  /** Accessible name: what the handle resizes. */
  label: string;
  /** Current size of the pane the handle controls, in px. It is the value of the separator. */
  value: number;
  min: number;
  max: number;
  /** Called with the new size while dragging and with each key press. */
  onChange: (value: number) => void;
  /**
   * Which way a drag to the right moves the size: `1` when moving right makes the pane larger, `-1`
   * when it makes it smaller. Arrow keys follow the same direction.
   */
  direction?: 1 | -1;
  /** Step of the arrow keys, in px. */
  step?: number;
  className?: string;
}

/**
 * The vertical handle between two panes. It is a focusable separator: drag it, or use the arrow keys
 * (20 px per press), Home and End. Its value is the size of the pane it controls.
 */
export function PaneSplitter({ label, value, min, max, onChange, direction = 1, step = 20, className }: PaneSplitterProps) {
  const drag = useRef<{ x: number; size: number } | undefined>(undefined);
  const clamp = (size: number) => Math.min(max, Math.max(min, Math.round(size)));

  const onPointerDown = (event: PointerEvent<HTMLDivElement>) => {
    event.preventDefault();
    event.currentTarget.setPointerCapture(event.pointerId);
    drag.current = { x: event.clientX, size: value };
  };
  const onPointerMove = (event: PointerEvent<HTMLDivElement>) => {
    if (!drag.current) return;
    onChange(clamp(drag.current.size + (event.clientX - drag.current.x) * direction));
  };
  const onPointerUp = (event: PointerEvent<HTMLDivElement>) => {
    drag.current = undefined;
    if (event.currentTarget.hasPointerCapture(event.pointerId)) event.currentTarget.releasePointerCapture(event.pointerId);
  };
  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    const grow = direction === 1 ? 'ArrowRight' : 'ArrowLeft';
    const shrink = direction === 1 ? 'ArrowLeft' : 'ArrowRight';
    if (event.key === grow) onChange(clamp(value + step));
    else if (event.key === shrink) onChange(clamp(value - step));
    else if (event.key === 'Home') onChange(min);
    else if (event.key === 'End') onChange(max);
    else return;
    event.preventDefault();
  };

  return (
    <div
      role="separator"
      aria-orientation="vertical"
      aria-label={label}
      aria-valuenow={Math.round(value)}
      aria-valuemin={min}
      aria-valuemax={max}
      tabIndex={0}
      data-pane-splitter
      onPointerDown={onPointerDown}
      onPointerMove={onPointerMove}
      onPointerUp={onPointerUp}
      onPointerCancel={onPointerUp}
      onKeyDown={onKeyDown}
      className={cn(
        'group relative z-10 w-2 shrink-0 cursor-col-resize touch-none select-none',
        // A wider hit area than the visible line, so the handle is easy to grab and meets the target size.
        'after:absolute after:inset-y-0 after:-inset-x-2 pointer-coarse:after:-inset-x-4',
        className,
      )}
    >
      <span aria-hidden className="bg-border group-hover:bg-canvas-selection group-focus-visible:bg-canvas-selection absolute inset-y-0 left-1/2 w-px -translate-x-1/2 group-hover:w-1" />
    </div>
  );
}
