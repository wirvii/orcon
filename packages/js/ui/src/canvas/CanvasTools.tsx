// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Cable, Keyboard, ListTree, Plus, Redo2, Trash2, Undo2 } from 'lucide-react';
import { useIntl } from 'react-intl';

import { IconButton } from '../components/icon-button/IconButton';
import { Separator } from '../components/separator/Separator';
import { Tooltip } from '../components/tooltip/Tooltip';
import { cn } from '../lib/cn';

import { canvasMessages } from './messages';

export interface CanvasToolsProps {
  editable?: boolean;
  canUndo?: boolean;
  canRedo?: boolean;
  canDelete?: boolean;
  /** The flow outline is visible. Reflected by the outline button. */
  outlineOpen?: boolean;
  onAdd?: () => void;
  onConnect?: () => void;
  onDelete?: () => void;
  onUndo?: () => void;
  onRedo?: () => void;
  onToggleOutline?: () => void;
  onShortcuts?: () => void;
  className?: string;
}

/**
 * The strip of tools above the canvas. Every graph operation has a button here, so nothing on the
 * canvas depends on a pointer: add, connect, delete, undo and redo, plus the flow outline and the
 * shortcut list. Each button names itself and shows its name in a tooltip.
 */
export function CanvasTools({ editable = true, canUndo = false, canRedo = false, canDelete = false, outlineOpen = false, onAdd, onConnect, onDelete, onUndo, onRedo, onToggleOutline, onShortcuts, className }: CanvasToolsProps) {
  const intl = useIntl();
  const button = (label: string, icon: typeof Plus, onClick: (() => void) | undefined, extra: { disabled?: boolean; pressed?: boolean } = {}) => (
    <Tooltip content={label}>
      <IconButton variant="ghost" icon={icon} label={label} onClick={onClick} disabled={extra.disabled} aria-pressed={extra.pressed} />
    </Tooltip>
  );
  return (
    <div
      role="toolbar"
      aria-label={intl.formatMessage(canvasMessages.toolbarLabel)}
      data-canvas-toolbar
      className={cn('bg-surface border-border flex flex-wrap items-center gap-1 border-b px-2 py-1', className)}
    >
      {editable ? (
        <>
          {button(intl.formatMessage(canvasMessages.addNode), Plus, onAdd)}
          {button(intl.formatMessage(canvasMessages.connectNodes), Cable, onConnect)}
          {button(intl.formatMessage(canvasMessages.deleteSelected), Trash2, onDelete, { disabled: !canDelete })}
          <Separator orientation="vertical" className="h-6" />
          {button(intl.formatMessage(canvasMessages.undo), Undo2, onUndo, { disabled: !canUndo })}
          {button(intl.formatMessage(canvasMessages.redo), Redo2, onRedo, { disabled: !canRedo })}
          <Separator orientation="vertical" className="h-6" />
        </>
      ) : null}
      {button(intl.formatMessage(canvasMessages.showOutline), ListTree, onToggleOutline, { pressed: outlineOpen })}
      {editable ? button(intl.formatMessage(canvasMessages.shortcuts), Keyboard, onShortcuts) : null}
    </div>
  );
}
