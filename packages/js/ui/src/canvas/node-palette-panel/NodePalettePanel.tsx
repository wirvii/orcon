// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Command } from 'cmdk';
import { Search, X } from 'lucide-react';
import { useEffect, useRef, useState, type KeyboardEvent, type ReactNode } from 'react';
import { useIntl } from 'react-intl';

import { IconButton } from '../../components/icon-button/IconButton';
import { Kbd } from '../../components/kbd/Kbd';
import { cn } from '../../lib/cn';
import { CommandResults } from '../../lib/command-list';
import { commandItem } from '../../lib/control';
import { NodeGlyph } from '../NodeGlyph';
import { canvasMessages } from '../messages';
import type { CatalogItem } from '../model';

/** The steps worth trying first after the step the new one goes behind. */
export interface PaletteSuggestions {
  /** Heading of the section ("Suggested after Webhook"). */
  heading: string;
  items: readonly CatalogItem[];
}

export interface NodePalettePanelProps {
  /** The nodes that can be added. */
  items: readonly CatalogItem[];
  /** "triggers" lists only triggers, for the first step of an empty flow. */
  filter?: 'triggers' | undefined;
  /** Shown above the groups while nothing is typed. */
  suggested?: PaletteSuggestions | undefined;
  /** The way out when a search finds nothing, below the notice. Receives what was typed. */
  renderEmpty?: ((query: string) => ReactNode) | undefined;
  onPick: (item: CatalogItem) => void;
  onClose: () => void;
  className?: string | undefined;
}

const headingClasses =
  '[&_[cmdk-group-heading]]:text-fg-muted [&_[cmdk-group-heading]]:px-2 [&_[cmdk-group-heading]]:pt-3 [&_[cmdk-group-heading]]:pb-1 [&_[cmdk-group-heading]]:text-xs [&_[cmdk-group-heading]]:font-semibold [&_[cmdk-group-heading]]:tracking-wide [&_[cmdk-group-heading]]:uppercase';

/**
 * The right-side panel that lists the nodes that can be added: a search field that has focus when it
 * opens, the steps worth trying first, a grouped list, arrow keys to move, Enter to insert and Escape to
 * close. When nothing matches the search it offers a way out (the host decides which). This is the visual
 * shell and the keyboard navigation; the node index, the search ranking and the grouping come from
 * the catalog and belong to the editor.
 */
export function NodePalettePanel({ items, filter, suggested, renderEmpty, onPick, onClose, className }: NodePalettePanelProps) {
  const intl = useIntl();
  const [search, setSearch] = useState('');
  const input = useRef<HTMLInputElement>(null);
  const listed = filter === 'triggers' ? items.filter((item) => item.kind === 'trigger') : items;
  const groups = Array.from(new Set(listed.map((item) => item.group)));
  const title = intl.formatMessage(filter === 'triggers' ? canvasMessages.paletteTriggers : canvasMessages.paletteTitle);
  // The suggested steps are among the listed ones: a trigger list or a trimmed list never offers one that is not there.
  const listedIds = new Set(listed.map((item) => item.id));
  const offered = (suggested?.items ?? []).filter((item) => listedIds.has(item.id));
  const showSuggested = suggested !== undefined && offered.length > 0 && search.trim() === '';

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

  const row = (item: CatalogItem, value: string, key: string) => (
    <Command.Item
      key={key}
      value={value}
      keywords={[...(item.keywords ?? []), ...(item.subtitle ? [item.subtitle] : [])]}
      onSelect={() => onPick(item)}
      className={cn(commandItem, 'h-auto gap-3 py-2')}
    >
      <span className="bg-surface-sunken border-border grid size-10 shrink-0 place-items-center rounded-md border p-2" aria-hidden>
        <NodeGlyph node={item} iconClassName="size-icon-20" className="size-icon-20" />
      </span>
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="font-medium wrap-anywhere">{item.name}</span>
        {item.subtitle ? <span className="text-fg-muted text-sm wrap-anywhere">{item.subtitle}</span> : null}
      </span>
    </Command.Item>
  );

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
      <div className="flex items-center justify-between gap-2 py-3 ps-4 pe-2">
        <h2 className="text-md font-semibold">{title}</h2>
        <IconButton variant="ghost" icon={X} label={intl.formatMessage(canvasMessages.paletteClose)} onClick={onClose} />
      </div>
      <Command label={intl.formatMessage(canvasMessages.paletteSearch)} className="flex min-h-0 flex-1 flex-col">
        <div className="border-control bg-surface mx-4 mb-2 flex items-center gap-2 rounded-md border px-3 focus-within:outline-solid focus-within:outline-2 focus-within:outline-offset-2 focus-within:outline-focus">
          <Search aria-hidden className="text-fg-muted size-icon-16 shrink-0" />
          <Command.Input
            ref={input}
            value={search}
            onValueChange={setSearch}
            placeholder={intl.formatMessage(canvasMessages.paletteSearch)}
            aria-label={intl.formatMessage(canvasMessages.paletteSearch)}
            className="text-fg placeholder:text-fg-subtle h-control min-h-target min-w-0 flex-1 bg-transparent text-base outline-hidden"
          />
        </div>
        <CommandResults
          emptyMessage={intl.formatMessage(canvasMessages.paletteEmpty)}
          emptyExtra={renderEmpty?.(search)}
          emptyClassName="text-fg-muted px-4 py-3 text-base"
          className="min-h-0 flex-1 overflow-y-auto px-2 pb-2"
        >
          {showSuggested ? (
            <Command.Group heading={suggested.heading} className={headingClasses}>
              {offered.map((item) => row(item, `suggested ${item.name}`, `suggested-${item.id}`))}
            </Command.Group>
          ) : null}
          {groups.map((group) => (
            <Command.Group key={group} heading={group} className={headingClasses}>
              {listed.filter((item) => item.group === group).map((item) => row(item, item.name, item.id))}
            </Command.Group>
          ))}
        </CommandResults>
        <p aria-hidden className="text-fg-muted border-border flex flex-wrap items-center gap-x-4 gap-y-1 border-t px-4 py-3 text-xs">
          <span>
            <Kbd>↑</Kbd> <Kbd>↓</Kbd> {intl.formatMessage(canvasMessages.paletteHintMove)}
          </span>
          <span>
            <Kbd>↵</Kbd> {intl.formatMessage(canvasMessages.paletteHintInsert)}
          </span>
          <span>
            <Kbd>{intl.formatMessage(canvasMessages.paletteKeyEscape)}</Kbd> {intl.formatMessage(canvasMessages.paletteHintClose)}
          </span>
        </p>
      </Command>
    </aside>
  );
}
