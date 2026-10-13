// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Command } from 'cmdk';
import { X } from 'lucide-react';
import { useEffect, useRef, useState, type KeyboardEvent } from 'react';
import { useIntl } from 'react-intl';

import { IconButton } from '../../components/icon-button/IconButton';
import { cn } from '../../lib/cn';
import { CommandResults } from '../../lib/command-list';
import { commandItem } from '../../lib/control';
import { NodeGlyph } from '../NodeGlyph';
import { canvasMessages } from '../messages';
import type { CatalogItem } from '../model';

export interface NodePalettePanelProps {
  /** The nodes that can be added. */
  items: readonly CatalogItem[];
  /** "triggers" lists only triggers, for the first step of an empty flow. */
  filter?: 'triggers' | undefined;
  onPick: (item: CatalogItem) => void;
  onClose: () => void;
  className?: string | undefined;
}

/**
 * The right-side panel that lists the nodes that can be added: a search field that has focus when it
 * opens, a grouped list, arrow keys to move, Enter to insert and Escape to close. This is the visual
 * shell and the keyboard navigation; the node index, the search ranking and the grouping come from
 * the catalog and belong to the editor.
 */
export function NodePalettePanel({ items, filter, onPick, onClose, className }: NodePalettePanelProps) {
  const intl = useIntl();
  const [search, setSearch] = useState('');
  const input = useRef<HTMLInputElement>(null);
  const listed = filter === 'triggers' ? items.filter((item) => item.kind === 'trigger') : items;
  const groups = Array.from(new Set(listed.map((item) => item.group)));
  const title = intl.formatMessage(filter === 'triggers' ? canvasMessages.paletteTriggers : canvasMessages.paletteTitle);

  useEffect(() => {
    input.current?.focus();
  }, []);

  const onKeyDown = (event: KeyboardEvent<HTMLElement>) => {
    event.stopPropagation();
    if (event.key === 'Escape') {
      event.preventDefault();
      onClose();
    }
  };

  return (
    <aside
      aria-label={title}
      data-node-palette
      onKeyDown={onKeyDown}
      className={cn(
        'nodrag nopan nowheel bg-surface-raised border-border absolute inset-y-0 right-0 z-30 flex w-canvas-palette-w max-w-full flex-col border-s shadow-lg',
        className,
      )}
    >
      <div className="border-border flex items-center justify-between gap-2 border-b py-2 ps-4 pe-2">
        <h2 className="text-md font-semibold">{title}</h2>
        <IconButton variant="ghost" icon={X} label={intl.formatMessage(canvasMessages.paletteClose)} onClick={onClose} />
      </div>
      <Command label={intl.formatMessage(canvasMessages.paletteSearch)} className="flex min-h-0 flex-1 flex-col">
        <Command.Input
          ref={input}
          value={search}
          onValueChange={setSearch}
          placeholder={intl.formatMessage(canvasMessages.paletteSearch)}
          aria-label={intl.formatMessage(canvasMessages.paletteSearch)}
          className="border-border text-fg placeholder:text-fg-subtle h-row min-h-target w-full border-b bg-transparent px-4 text-base outline-hidden"
        />
        <CommandResults
          emptyMessage={intl.formatMessage(canvasMessages.paletteEmpty)}
          emptyClassName="text-fg-muted px-4 py-3 text-base"
          className="min-h-0 flex-1 overflow-y-auto p-2"
        >
          {groups.map((group) => (
            <Command.Group
              key={group}
              heading={group}
              className="[&_[cmdk-group-heading]]:text-fg-muted [&_[cmdk-group-heading]]:px-2 [&_[cmdk-group-heading]]:py-1 [&_[cmdk-group-heading]]:text-xs [&_[cmdk-group-heading]]:font-semibold"
            >
              {listed
                .filter((item) => item.group === group)
                .map((item) => (
                  <Command.Item
                    key={item.id}
                    value={item.name}
                    keywords={[...(item.keywords ?? []), ...(item.subtitle ? [item.subtitle] : [])]}
                    onSelect={() => onPick(item)}
                    className={cn(commandItem, 'h-auto gap-3 py-2')}
                  >
                    <span className="bg-surface-sunken border-border grid size-icon-24 shrink-0 place-items-center rounded-md border p-1" aria-hidden>
                      <NodeGlyph node={item} iconClassName="size-icon-16" className="size-icon-16" />
                    </span>
                    <span className="flex min-w-0 flex-1 flex-col">
                      <span className="font-medium wrap-anywhere">{item.name}</span>
                      {item.subtitle ? <span className="text-fg-muted text-sm wrap-anywhere">{item.subtitle}</span> : null}
                    </span>
                  </Command.Item>
                ))}
            </Command.Group>
          ))}
        </CommandResults>
      </Command>
    </aside>
  );
}
