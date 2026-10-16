// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { IconButton, Separator, Tooltip } from '@orcon/ui';
import { Command, Keyboard, ListTree, Plus, Redo2, Save, Search, StickyNote, Undo2, Wand2 } from 'lucide-react';
import { useRef, useState, type KeyboardEvent } from 'react';
import { useIntl, type MessageDescriptor } from 'react-intl';

import { useEditorStore } from '../store/editor-context';

import { editorMessages } from './messages';
import type { EditorCommands } from './use-editor-commands';

export interface EditorToolbarProps {
  /** The editing buttons need a tablet or a computer; a phone shows the flow only. */
  editable: boolean;
  outlineOpen: boolean;
  commands: EditorCommands;
  onToggleOutline: () => void;
  onFind: () => void;
  onCommands: () => void;
  onShortcuts: () => void;
  onSave: () => void;
  className?: string | undefined;
}

interface Tool {
  label: MessageDescriptor;
  icon: typeof Plus;
  onClick: () => void;
  disabled?: boolean;
  pressed?: boolean;
}

/**
 * The tools that are not on the canvas itself: add, notes, history, tidy up, find, commands, save, the outline
 * and the shortcuts. It floats over the canvas, at its top left, as a WAI-ARIA toolbar: one tab stop, the arrow
 * keys move between the buttons.
 */
export function EditorToolbar({ editable, outlineOpen, commands, onToggleOutline, onFind, onCommands, onShortcuts, onSave, className }: EditorToolbarProps) {
  const intl = useIntl();
  const canUndo = useEditorStore((state) => state.history.past.length > 0);
  const canRedo = useEditorStore((state) => state.history.future.length > 0);

  // The tab stop is the button that had focus last (or the first one that can be used).
  const toolbar = useRef<HTMLDivElement>(null);
  const [current, setCurrent] = useState<string | undefined>(undefined);
  const enabled = () => Array.from(toolbar.current?.querySelectorAll<HTMLButtonElement>('button:not(:disabled)') ?? []);
  const onToolbarKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    const buttons = enabled();
    const index = buttons.findIndex((button) => button === document.activeElement);
    if (index < 0) return;
    const last = buttons.length - 1;
    const target = event.key === 'ArrowRight' ? (index === last ? 0 : index + 1) : event.key === 'ArrowLeft' ? (index === 0 ? last : index - 1) : event.key === 'Home' ? 0 : event.key === 'End' ? last : undefined;
    if (target === undefined) return;
    event.preventDefault();
    buttons[target]?.focus();
  };

  const sections: Tool[][] = [
    ...(editable
      ? [
          [
            { label: editorMessages.addNode, icon: Plus, onClick: commands.addNode },
            { label: editorMessages.addNote, icon: StickyNote, onClick: commands.addNote },
          ],
          [
            { label: editorMessages.undo, icon: Undo2, onClick: commands.undo, disabled: !canUndo },
            { label: editorMessages.redo, icon: Redo2, onClick: commands.redo, disabled: !canRedo },
            { label: editorMessages.tidyUp, icon: Wand2, onClick: () => void commands.tidyUp() },
          ],
          [
            { label: editorMessages.find, icon: Search, onClick: onFind },
            { label: editorMessages.commands, icon: Command, onClick: onCommands },
            { label: editorMessages.saveNow, icon: Save, onClick: onSave },
          ],
        ]
      : []),
    [
      { label: editorMessages.showOutline, icon: ListTree, onClick: onToggleOutline, pressed: outlineOpen },
      ...(editable ? [{ label: editorMessages.shortcuts, icon: Keyboard, onClick: onShortcuts }] : []),
    ],
  ];
  const all = sections.flat();
  const stop = all.find((tool) => tool.label.id === current && tool.disabled !== true)?.label.id ?? all.find((tool) => tool.disabled !== true)?.label.id;

  return (
    <div
      ref={toolbar}
      role="toolbar"
      aria-label={intl.formatMessage(editorMessages.toolbar)}
      onKeyDown={onToolbarKeyDown}
      onFocus={(event) => {
        const id = event.target instanceof HTMLElement ? event.target.dataset['tool'] : undefined;
        if (id) setCurrent(id);
      }}
      className={`bg-surface-raised border-border flex flex-wrap items-center gap-1 rounded-lg border p-1 shadow-sm ${className ?? ''}`}
    >
      {sections.map((tools, index) => (
        <div key={tools[0]?.label.id} className="flex items-center gap-1">
          {index > 0 ? <Separator orientation="vertical" className="h-6" /> : null}
          {tools.map((tool) => {
            const label = intl.formatMessage(tool.label);
            return (
              <Tooltip key={tool.label.id} content={label}>
                <IconButton
                  variant="ghost"
                  icon={tool.icon}
                  label={label}
                  onClick={tool.onClick}
                  disabled={tool.disabled}
                  aria-pressed={tool.pressed}
                  tabIndex={tool.label.id === stop ? 0 : -1}
                  data-tool={tool.label.id}
                />
              </Tooltip>
            );
          })}
        </div>
      ))}
    </div>
  );
}
