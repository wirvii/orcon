// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Command } from 'cmdk';
import { Dialog } from 'radix-ui';
import { useState } from 'react';
import { useIntl } from 'react-intl';

import { cn } from '../lib/cn';
import { CommandResults } from '../lib/command-list';
import { commandItem, modalPanel, scrim } from '../lib/control';

import { canvasMessages } from './messages';
import type { CatalogItem } from './model';

export interface NodePaletteProps {
  catalog: readonly CatalogItem[];
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onPick: (item: CatalogItem) => void;
}

/**
 * The searchable list of nodes that can be added to the flow. It is the keyboard
 * way to place a node: open it with A, type to filter, Enter adds.
 */
export function NodePalette({ catalog, open, onOpenChange, onPick }: NodePaletteProps) {
  const intl = useIntl();
  const [search, setSearch] = useState('');
  const groups = Array.from(new Set(catalog.map((item) => item.group)));

  const close = (next: boolean) => {
    onOpenChange(next);
    if (!next) setSearch('');
  };

  return (
    <Dialog.Root open={open} onOpenChange={close}>
      <Dialog.Portal>
        <Dialog.Overlay className={scrim} />
        <div className="pointer-events-none fixed inset-0 z-modal flex items-start justify-center p-4 pt-16">
          <Dialog.Content aria-describedby={undefined} className={cn(modalPanel, 'pointer-events-auto w-full max-w-screen-sm overflow-hidden rounded-xl')}>
            <Dialog.Title className="sr-only">{intl.formatMessage(canvasMessages.paletteTitle)}</Dialog.Title>
            <Command label={intl.formatMessage(canvasMessages.paletteSearch)}>
              <Command.Input
                value={search}
                onValueChange={setSearch}
                placeholder={intl.formatMessage(canvasMessages.paletteSearch)}
                aria-label={intl.formatMessage(canvasMessages.paletteSearch)}
                className="border-border text-fg placeholder:text-fg-subtle h-row min-h-target w-full border-b bg-transparent px-control-x text-base outline-hidden"
              />
              <CommandResults
                emptyMessage={intl.formatMessage(canvasMessages.paletteEmpty)}
                emptyClassName="text-fg-muted px-control-x py-3 text-base"
                className="max-h-available overflow-y-auto p-1"
              >
                {groups.map((group) => (
                  <Command.Group
                    key={group}
                    heading={group}
                    className="[&_[cmdk-group-heading]]:text-fg-muted [&_[cmdk-group-heading]]:px-control-x [&_[cmdk-group-heading]]:py-1 [&_[cmdk-group-heading]]:text-sm [&_[cmdk-group-heading]]:font-medium"
                  >
                    {catalog
                      .filter((item) => item.group === group)
                      .map((item) => (
                        <Command.Item
                          key={item.id}
                          value={item.name}
                          keywords={[...(item.keywords ?? []), ...(item.summary ? [item.summary] : [])]}
                          onSelect={() => {
                            close(false);
                            onPick(item);
                          }}
                          className={cn(commandItem, 'h-auto py-1')}
                        >
                          <span className="flex min-w-0 flex-1 flex-col">
                            <span className="break-words">{item.name}</span>
                            {item.summary ? <span className="text-fg-muted text-sm break-words">{item.summary}</span> : null}
                          </span>
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
