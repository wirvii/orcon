// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Plus, Trash2 } from 'lucide-react';
import { useIntl } from 'react-intl';

import { cn } from '../../lib/cn';
import { canvasMessages } from '../messages';

export interface EdgeActionsProps {
  /** Name of the node the connection starts at. */
  source: string;
  /** Name of the node the connection ends at. */
  target: string;
  /** Insert a node in the middle of the connection. */
  onAdd?: (() => void) | undefined;
  onDelete?: (() => void) | undefined;
  /** Reachable with Tab. */
  tabbable?: boolean | undefined;
  /** Pointer enter and leave, so the canvas keeps the bar open while the pointer is on it. */
  onPointerEnter?: (() => void) | undefined;
  onPointerLeave?: (() => void) | undefined;
  className?: string | undefined;
}

const button =
  'nodrag nopan text-fg hover:bg-surface-sunken relative grid size-icon-24 min-h-target min-w-target place-items-center rounded-md p-0';

/**
 * The two actions that appear on a hovered or focused connection: add a node in the middle of it, or
 * delete it. They are real buttons with accessible names; on a focused connection Enter and Delete do
 * the same without moving to the bar.
 */
export function EdgeActions({ source, target, onAdd, onDelete, tabbable = false, onPointerEnter, onPointerLeave, className }: EdgeActionsProps) {
  const intl = useIntl();
  return (
    <div
      role="toolbar"
      aria-label={intl.formatMessage(canvasMessages.edgeActionsFor, { source, target })}
      data-edge-actions
      onPointerEnter={onPointerEnter}
      onPointerLeave={onPointerLeave}
      className={cn('bg-surface-raised border-canvas-node-border flex gap-1 rounded-md border p-1 shadow-md', className)}
    >
      <button type="button" tabIndex={tabbable ? 0 : -1} aria-label={intl.formatMessage(canvasMessages.edgeAddHere)} title={intl.formatMessage(canvasMessages.edgeAddHere)} onClick={onAdd} className={button}>
        <Plus aria-hidden className="size-icon-16" />
      </button>
      <button
        type="button"
        tabIndex={tabbable ? 0 : -1}
        aria-label={intl.formatMessage(canvasMessages.edgeDelete)}
        title={intl.formatMessage(canvasMessages.edgeDelete)}
        onClick={onDelete}
        className={cn(button, 'hover:bg-status-danger-bg hover:text-status-danger')}
      >
        <Trash2 aria-hidden className="size-icon-16" />
      </button>
    </div>
  );
}
