// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Command } from 'cmdk';
import { Dialog } from 'radix-ui';
import { useEffect, useState } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { cn } from '../../lib/cn';
import { commandItem, modalPanel, scrim } from '../../lib/control';
import { CommandResults } from '../../lib/command-list';
import { KbdCombination } from '../kbd/Kbd';

const messages = defineMessages({
  title: {
    id: 'palette.title',
    defaultMessage: 'Command palette',
    description: 'Accessible name of the dialog where a person types to find and run a command.',
  },
  placeholder: {
    id: 'palette.placeholder',
    defaultMessage: 'Type a command or search',
    description: 'Placeholder and accessible name of the search box in the command palette.',
  },
  empty: { id: 'palette.empty', defaultMessage: 'No matching commands', description: 'Shown in the command palette when nothing matches what was typed.' },
});

export interface PaletteCommand {
  id: string;
  label: string;
  /** Heading the command is listed under. */
  group?: string | undefined;
  /** Keys, shown on the right, for a command that has a shortcut. */
  shortcut?: readonly string[] | undefined;
  /** Extra words that also match the search. */
  keywords?: readonly string[] | undefined;
  onSelect: () => void;
}

export interface CommandPaletteProps {
  commands: readonly PaletteCommand[];
  open?: boolean | undefined;
  defaultOpen?: boolean | undefined;
  onOpenChange?: ((open: boolean) => void) | undefined;
  /** Open and close with Ctrl+K or Cmd+K. Default true. */
  enableShortcut?: boolean | undefined;
  /** Text typed in the search box when it opens. Only for stories and tests. */
  defaultSearch?: string | undefined;
}

/**
 * A keyboard-first way to run any command: open it with Ctrl or Cmd + K, type to
 * filter, arrow keys to move, Enter to run, Escape to close. It is a modal dialog
 * with a combobox and a listbox inside.
 */
export function CommandPalette({ commands, open: controlledOpen, defaultOpen = false, onOpenChange, enableShortcut = true, defaultSearch = '' }: CommandPaletteProps) {
  const intl = useIntl();
  const [innerOpen, setInnerOpen] = useState(defaultOpen);
  const open = controlledOpen ?? innerOpen;
  const [search, setSearch] = useState(defaultSearch);

  const setOpen = (next: boolean) => {
    if (controlledOpen === undefined) setInnerOpen(next);
    onOpenChange?.(next);
    if (!next) setSearch('');
  };

  useEffect(() => {
    if (!enableShortcut) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key.toLowerCase() === 'k' && (event.metaKey || event.ctrlKey)) {
        event.preventDefault();
        setOpen(!open);
      }
    };
    document.addEventListener('keydown', onKeyDown);
    return () => document.removeEventListener('keydown', onKeyDown);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- setOpen is rebuilt from the listed values every render
  }, [enableShortcut, open, controlledOpen, onOpenChange]);

  const groups = Array.from(new Set(commands.map((command) => command.group ?? '')));

  return (
    <Dialog.Root open={open} onOpenChange={setOpen}>
      <Dialog.Portal>
        <Dialog.Overlay className={scrim} />
        <div className="pointer-events-none fixed inset-0 z-modal flex items-start justify-center p-4 pt-16">
          <Dialog.Content aria-describedby={undefined} className={cn(modalPanel, 'pointer-events-auto w-full max-w-screen-sm overflow-hidden rounded-xl')}>
            <Dialog.Title className="sr-only">{intl.formatMessage(messages.title)}</Dialog.Title>
            {/* cmdk names its input from this label, so it carries the search prompt. */}
            <Command label={intl.formatMessage(messages.placeholder)}>
              <Command.Input
                value={search}
                onValueChange={setSearch}
                placeholder={intl.formatMessage(messages.placeholder)}
                aria-label={intl.formatMessage(messages.placeholder)}
                className="border-border text-fg placeholder:text-fg-subtle h-row min-h-target w-full border-b bg-transparent px-control-x text-base outline-hidden"
              />
              <CommandResults
                emptyMessage={intl.formatMessage(messages.empty)}
                emptyClassName="text-fg-muted px-control-x py-3 text-base"
                className="max-h-available overflow-y-auto p-1"
              >
                {groups.map((group) => (
                  <Command.Group
                    key={group || 'ungrouped'}
                    heading={group || undefined}
                    className="[&_[cmdk-group-heading]]:text-fg-muted [&_[cmdk-group-heading]]:px-control-x [&_[cmdk-group-heading]]:py-1 [&_[cmdk-group-heading]]:text-sm [&_[cmdk-group-heading]]:font-medium"
                  >
                    {commands
                      .filter((command) => (command.group ?? '') === group)
                      .map((command) => (
                        <Command.Item
                          key={command.id}
                          value={command.label}
                          {...(command.keywords ? { keywords: [...command.keywords] } : {})}
                          onSelect={() => {
                            setOpen(false);
                            command.onSelect();
                          }}
                          className={commandItem}
                        >
                          <span className="min-w-0 flex-1 wrap-anywhere">{command.label}</span>
                          {command.shortcut ? <KbdCombination keys={command.shortcut} /> : null}
                        </Command.Item>
                      ))}
                  </Command.Group>
                ))}
              </CommandResults>
            </Command>
          </Dialog.Content>
        </div>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
