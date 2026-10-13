// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { useIntl } from 'react-intl';
import type { MessageDescriptor } from 'react-intl';

import { Dialog } from '../components/dialog/Dialog';
import { KbdCombination } from '../components/kbd/Kbd';

import { canvasMessages } from './messages';

interface Shortcut {
  keys: readonly string[];
  description: MessageDescriptor;
}

const isMac = () => typeof navigator !== 'undefined' && /mac|iphone|ipad/i.test(navigator.userAgent);

export function shortcutList(): readonly Shortcut[] {
  const mod = isMac() ? 'Cmd' : 'Ctrl';
  return [
    { keys: ['←', '↑', '→', '↓'], description: canvasMessages.shortcutsMove },
    { keys: ['Enter'], description: canvasMessages.shortcutsOpen },
    { keys: ['Space'], description: canvasMessages.shortcutsSelect },
    { keys: ['Shift', '←↑→↓'], description: canvasMessages.shortcutsNudge },
    { keys: ['Alt', 'Shift', '←↑→↓'], description: canvasMessages.shortcutsNudgeFar },
    { keys: ['A'], description: canvasMessages.shortcutsAdd },
    { keys: ['C'], description: canvasMessages.shortcutsConnect },
    { keys: ['R'], description: canvasMessages.shortcutsRun },
    { keys: ['D'], description: canvasMessages.shortcutsDisable },
    { keys: ['P'], description: canvasMessages.shortcutsPin },
    { keys: ['F2'], description: canvasMessages.shortcutsRename },
    { keys: ['Shift', 'S'], description: canvasMessages.shortcutsSticky },
    { keys: [mod, 'Shift', '←↑→↓'], description: canvasMessages.shortcutsResizeNote },
    { keys: ['Enter', 'Delete'], description: canvasMessages.shortcutsEdge },
    { keys: ['Delete'], description: canvasMessages.shortcutsDelete },
    { keys: ['+', '-', '0', '1'], description: canvasMessages.shortcutsZoom },
    { keys: [mod, 'Z'], description: canvasMessages.shortcutsUndo },
    { keys: [mod, 'Shift', 'Z'], description: canvasMessages.shortcutsRedo },
    { keys: [mod, 'A'], description: canvasMessages.shortcutsSelectAll },
    { keys: ['Esc'], description: canvasMessages.shortcutsClear },
    { keys: ['?'], description: canvasMessages.shortcutsHelp },
  ];
}

export interface ShortcutsDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

/** The keyboard shortcuts of the canvas, as a description list. */
export function ShortcutsDialog({ open, onOpenChange }: ShortcutsDialogProps) {
  const intl = useIntl();
  return (
    <Dialog open={open} onOpenChange={onOpenChange} title={intl.formatMessage(canvasMessages.shortcutsTitle)}>
      <dl className="flex flex-col gap-2">
        {shortcutList().map((shortcut) => (
          <div key={shortcut.description.id} className="border-border flex flex-wrap items-center justify-between gap-2 border-b pb-2">
            <dt className="wrap-anywhere">{intl.formatMessage(shortcut.description)}</dt>
            <dd>
              <KbdCombination keys={shortcut.keys} />
            </dd>
          </div>
        ))}
      </dl>
    </Dialog>
  );
}
