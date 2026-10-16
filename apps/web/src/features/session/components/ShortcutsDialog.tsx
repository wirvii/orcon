// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Dialog, Kbd } from '@orcon/ui';
import { defineMessages, useIntl } from 'react-intl';

const messages = defineMessages({
  title: {
    id: 'session.shortcuts.title',
    defaultMessage: 'Keyboard shortcuts',
    description: 'Title of the dialog that lists the keyboard shortcuts of the app.',
  },
  description: {
    id: 'session.shortcuts.description',
    defaultMessage: 'They work anywhere in the app, except while you are typing in a field.',
    description: 'One line under the title of the shortcuts dialog: where the shortcuts work.',
  },
  createFlow: { id: 'session.shortcuts.createFlow', defaultMessage: 'Create a flow', description: 'What the key N does.' },
  collapse: { id: 'session.shortcuts.collapse', defaultMessage: 'Collapse or expand the sidebar', description: 'What the key [ does.' },
  shortcuts: { id: 'session.shortcuts.list', defaultMessage: 'Show this list', description: 'What the key ? does.' },
  close: { id: 'session.shortcuts.close', defaultMessage: 'Close a dialog or a menu', description: 'What the key Escape does.' },
});

export interface ShortcutsDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

/** The keyboard shortcuts of the app. Only keys that do something today are listed. */
export function ShortcutsDialog({ open, onOpenChange }: ShortcutsDialogProps) {
  const intl = useIntl();
  const rows = [
    { key: 'N', text: intl.formatMessage(messages.createFlow) },
    { key: '[', text: intl.formatMessage(messages.collapse) },
    { key: '?', text: intl.formatMessage(messages.shortcuts) },
    { key: 'Esc', text: intl.formatMessage(messages.close) },
  ];
  return (
    <Dialog open={open} onOpenChange={onOpenChange} title={intl.formatMessage(messages.title)} description={intl.formatMessage(messages.description)}>
      <ul className="flex flex-col gap-2">
        {rows.map((row) => (
          <li key={row.key} className="flex items-center justify-between gap-3">
            <span>{row.text}</span>
            <Kbd>{row.key}</Kbd>
          </li>
        ))}
      </ul>
    </Dialog>
  );
}
