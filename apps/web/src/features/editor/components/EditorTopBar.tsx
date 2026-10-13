// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { IconButton, Separator, Tooltip } from '@orcon/ui';
import { Command, Keyboard, ListTree, Plus, Redo2, Save, Search, StickyNote, Undo2, Wand2 } from 'lucide-react';
import { useIntl, type MessageDescriptor } from 'react-intl';

import type { SaveStatus } from '../model/autosave';
import { useEditorStore } from '../store/editor-context';

import { editorMessages } from './messages';
import { SaveIndicator } from './SaveIndicator';
import type { EditorCommands } from './use-editor-commands';

export interface EditorTopBarProps {
  /** The editing buttons need a tablet or a computer; a phone shows the flow only. */
  editable: boolean;
  status: SaveStatus;
  outlineOpen: boolean;
  commands: EditorCommands;
  onToggleOutline: () => void;
  onFind: () => void;
  onCommands: () => void;
  onShortcuts: () => void;
  onSave: () => void;
}

interface Tool {
  label: MessageDescriptor;
  icon: typeof Plus;
  onClick: () => void;
  disabled?: boolean;
  pressed?: boolean;
}

/** The name of the flow, the state of its draft and the tools that are not on the canvas itself. */
export function EditorTopBar({ editable, status, outlineOpen, commands, onToggleOutline, onFind, onCommands, onShortcuts, onSave }: EditorTopBarProps) {
  const intl = useIntl();
  const name = useEditorStore((state) => state.document.name);
  const canUndo = useEditorStore((state) => state.history.past.length > 0);
  const canRedo = useEditorStore((state) => state.history.future.length > 0);

  const group = (tools: Tool[]) =>
    tools.map((tool) => {
      const label = intl.formatMessage(tool.label);
      return (
        <Tooltip key={tool.label.id} content={label}>
          <IconButton variant="ghost" icon={tool.icon} label={label} onClick={tool.onClick} disabled={tool.disabled} aria-pressed={tool.pressed} />
        </Tooltip>
      );
    });

  return (
    <header className="border-border flex flex-wrap items-center gap-x-3 gap-y-2 border-b px-4 py-2">
      <h1 className="text-md me-auto min-w-0 font-semibold wrap-anywhere">{name}</h1>
      {editable ? <SaveIndicator status={status} /> : null}
      <div role="toolbar" aria-label={intl.formatMessage(editorMessages.toolbar)} className="flex flex-wrap items-center gap-1">
        {editable ? (
          <>
            {group([
              { label: editorMessages.addNode, icon: Plus, onClick: commands.addNode },
              { label: editorMessages.addNote, icon: StickyNote, onClick: commands.addNote },
            ])}
            <Separator orientation="vertical" className="h-6" />
            {group([
              { label: editorMessages.undo, icon: Undo2, onClick: commands.undo, disabled: !canUndo },
              { label: editorMessages.redo, icon: Redo2, onClick: commands.redo, disabled: !canRedo },
              { label: editorMessages.tidyUp, icon: Wand2, onClick: () => void commands.tidyUp() },
            ])}
            <Separator orientation="vertical" className="h-6" />
            {group([
              { label: editorMessages.find, icon: Search, onClick: onFind },
              { label: editorMessages.commands, icon: Command, onClick: onCommands },
              { label: editorMessages.saveNow, icon: Save, onClick: onSave },
            ])}
            <Separator orientation="vertical" className="h-6" />
          </>
        ) : null}
        {group([{ label: editorMessages.showOutline, icon: ListTree, onClick: onToggleOutline, pressed: outlineOpen }])}
        {editable ? group([{ label: editorMessages.shortcuts, icon: Keyboard, onClick: onShortcuts }]) : null}
      </div>
    </header>
  );
}
