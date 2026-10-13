// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Ellipsis, Play, Power, Trash2, Zap, type LucideIcon } from 'lucide-react';
import { useIntl } from 'react-intl';

import { DropdownMenu, type MenuEntry } from '../../components/dropdown-menu/DropdownMenu';
import { Tooltip } from '../../components/tooltip/Tooltip';
import { cn } from '../../lib/cn';
import { canvasMessages } from '../messages';

export interface NodeToolbarProps {
  /** The node name: it names the toolbar. */
  name: string;
  /** A trigger node starts with "Execute workflow". */
  trigger?: boolean | undefined;
  /** The node is disabled: the power button turns it back on. */
  disabled?: boolean | undefined;
  pinned?: boolean | undefined;
  /** A node that cannot run (an unsupported node) has no run button. */
  canRun?: boolean | undefined;
  /** Reachable with Tab. Only the node that holds the roving tab stop sets it. */
  tabbable?: boolean | undefined;
  onExecuteWorkflow?: (() => void) | undefined;
  onRunStep?: (() => void) | undefined;
  onToggleDisabled?: (() => void) | undefined;
  onDelete?: (() => void) | undefined;
  onRename?: (() => void) | undefined;
  onDuplicate?: (() => void) | undefined;
  onTogglePin?: (() => void) | undefined;
  onCopy?: (() => void) | undefined;
  onOpenDetails?: (() => void) | undefined;
  className?: string | undefined;
}

const buttonLook =
  'nodrag nopan text-fg hover:bg-surface-sunken grid size-canvas-toolbar-button place-items-center rounded-md p-0 min-h-target min-w-target';

interface ToolbarButtonProps {
  icon: LucideIcon;
  label: string;
  shortcut?: string | undefined;
  tabbable: boolean;
  onClick?: (() => void) | undefined;
  danger?: boolean | undefined;
  primary?: boolean | undefined;
}

function ToolbarButton({ icon: Icon, label, shortcut, tabbable, onClick, danger, primary }: ToolbarButtonProps) {
  return (
    <Tooltip content={shortcut ? `${label} (${shortcut})` : label}>
      <button
        type="button"
        aria-label={label}
        tabIndex={tabbable ? 0 : -1}
        onClick={(event) => {
          event.stopPropagation();
          onClick?.();
        }}
        className={cn(buttonLook, danger && 'hover:bg-status-danger-bg hover:text-status-danger', primary && 'text-fg-accent')}
      >
        <Icon aria-hidden className="size-icon-16" />
      </button>
    </Tooltip>
  );
}

/**
 * The quick actions that float 40 px above a node on hover, focus and selection: run the step (or
 * execute the workflow, for a trigger), disable or enable, delete, and a menu with rename, duplicate,
 * pin, copy and open details. Every button has an accessible name and a tooltip with its shortcut.
 */
export function NodeToolbar({
  name,
  trigger = false,
  disabled = false,
  pinned = false,
  canRun = true,
  tabbable = true,
  onExecuteWorkflow,
  onRunStep,
  onToggleDisabled,
  onDelete,
  onRename,
  onDuplicate,
  onTogglePin,
  onCopy,
  onOpenDetails,
  className,
}: NodeToolbarProps) {
  const intl = useIntl();
  const menu: MenuEntry[] = [
    { type: 'item', label: intl.formatMessage(canvasMessages.actionRename), onSelect: onRename },
    { type: 'item', label: intl.formatMessage(canvasMessages.actionDuplicate), onSelect: onDuplicate },
    { type: 'item', label: intl.formatMessage(pinned ? canvasMessages.actionUnpin : canvasMessages.actionPin), onSelect: onTogglePin },
    { type: 'item', label: intl.formatMessage(canvasMessages.actionCopy), onSelect: onCopy },
    { type: 'separator' },
    { type: 'item', label: intl.formatMessage(canvasMessages.actionOpenDetails), onSelect: onOpenDetails },
  ];
  return (
    <div
      role="toolbar"
      aria-label={intl.formatMessage(canvasMessages.toolbarFor, { name })}
      data-node-toolbar
      className={cn(
        'bg-surface-raised border-canvas-node-border absolute bottom-full left-1/2 z-30 mb-2 flex -translate-x-1/2 gap-1 rounded-md border p-1 shadow-md',
        // An invisible bridge keeps the toolbar reachable when the pointer travels from the tile to it.
        'after:absolute after:inset-x-0 after:top-full after:h-3',
        className,
      )}
    >
      {trigger ? <ToolbarButton icon={Zap} label={intl.formatMessage(canvasMessages.actionExecuteWorkflow)} tabbable={tabbable} onClick={onExecuteWorkflow} primary /> : null}
      {canRun ? <ToolbarButton icon={Play} label={intl.formatMessage(canvasMessages.actionRunStep)} shortcut="R" tabbable={tabbable} onClick={onRunStep} /> : null}
      <ToolbarButton
        icon={Power}
        label={intl.formatMessage(disabled ? canvasMessages.actionEnable : canvasMessages.actionDisable)}
        shortcut="D"
        tabbable={tabbable}
        onClick={onToggleDisabled}
      />
      <ToolbarButton icon={Trash2} label={intl.formatMessage(canvasMessages.actionDelete)} shortcut="Del" tabbable={tabbable} onClick={onDelete} danger />
      <DropdownMenu
        items={menu}
        trigger={
          <button
            type="button"
            aria-label={intl.formatMessage(canvasMessages.actionMore)}
            tabIndex={tabbable ? 0 : -1}
            onClick={(event) => event.stopPropagation()}
            className={buttonLook}
          >
            <Ellipsis aria-hidden className="size-icon-16" />
          </button>
        }
      />
    </div>
  );
}
