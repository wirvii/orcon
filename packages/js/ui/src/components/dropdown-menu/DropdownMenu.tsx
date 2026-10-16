// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Check, ExternalLink, type LucideIcon } from 'lucide-react';
import { DropdownMenu as MenuPrimitive } from 'radix-ui';
import type { ComponentProps, ReactElement, ReactNode } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { cn } from '../../lib/cn';
import { floatingPanel, menuItem } from '../../lib/control';

const messages = defineMessages({
  opensInNewTab: {
    id: 'common.link.opensInNewTab',
    defaultMessage: '(opens in a new tab)',
    description: 'Screen reader text appended to a link that opens in a new browser tab.',
  },
});

const externalProps = { target: '_blank', rel: 'noopener noreferrer' };

export type MenuEntry =
  | {
      type: 'item';
      label: string;
      onSelect?: (() => void) | undefined;
      disabled?: boolean | undefined;
      /** Irreversible or risky action. Shown in the danger color with the label unchanged; confirm it with an AlertDialog. */
      destructive?: boolean | undefined;
      /** Drawn before the label. */
      icon?: LucideIcon | undefined;
      /** Something custom before the label, such as a letter tile. It replaces `icon`. */
      leading?: ReactNode | undefined;
      /** The key that does the same thing without the menu, shown at the end. */
      shortcut?: string | undefined;
      /** The choice in force, among items that are a choice: a check mark at the end and `aria-checked`. */
      selected?: boolean | undefined;
    }
  | {
      /** An item that goes to an address. With `external` it opens a new tab and says so, with an icon and to screen readers. */
      type: 'link';
      label: string;
      href: string;
      external?: boolean | undefined;
      icon?: LucideIcon | undefined;
      shortcut?: string | undefined;
      /** Called on a plain click, so a router can change the address without reloading. */
      onSelect?: (() => void) | undefined;
    }
  | { type: 'separator' }
  | { type: 'label'; label: string }
  /** A block that is not an item, such as the name and email at the top of an account menu. */
  | { type: 'content'; id: string; content: ReactNode };

export interface DropdownMenuProps extends Pick<ComponentProps<typeof MenuPrimitive.Root>, 'open' | 'defaultOpen' | 'onOpenChange'> {
  /** The button that opens the menu. */
  trigger: ReactElement;
  items: readonly MenuEntry[];
  align?: ComponentProps<typeof MenuPrimitive.Content>['align'] | undefined;
  side?: ComponentProps<typeof MenuPrimitive.Content>['side'] | undefined;
  /** `trigger` (default) is at least as wide as the button; `panel` is at least the width of a side panel. */
  width?: 'trigger' | 'panel' | undefined;
}

/** A list of actions opened from a button. Arrow keys move, Enter selects, Escape closes and returns focus. */
export function DropdownMenu({ trigger, items, align = 'end', side, width = 'trigger', ...root }: DropdownMenuProps) {
  const intl = useIntl();
  return (
    // Non-modal: the rest of the page stays exposed, which keeps the trigger reachable for assistive technology.
    <MenuPrimitive.Root modal={false} {...root}>
      <MenuPrimitive.Trigger asChild>{trigger}</MenuPrimitive.Trigger>
      <MenuPrimitive.Portal>
        <MenuPrimitive.Content
          align={align}
          side={side}
          sideOffset={4}
          collisionPadding={8}
          className={cn(floatingPanel, width === 'panel' ? 'min-w-panel-width' : 'min-w-trigger', 'max-h-available overflow-y-auto p-1')}
        >
          {items.map((entry, index) => {
            if (entry.type === 'separator') {
              return <MenuPrimitive.Separator key={`separator-${index}`} className="border-strong my-1 border-t" />;
            }
            if (entry.type === 'label') {
              return (
                <MenuPrimitive.Label key={`label-${entry.label}`} className="text-fg-muted px-control-x py-1 text-sm font-medium">
                  {entry.label}
                </MenuPrimitive.Label>
              );
            }
            if (entry.type === 'content') {
              return (
                <div key={entry.id} className="px-control-x py-2">
                  {entry.content}
                </div>
              );
            }
            if (entry.type === 'link') {
              const Icon = entry.icon;
              return (
                <MenuPrimitive.Item key={entry.href + entry.label} asChild className={menuItem}>
                  <a
                    href={entry.href}
                    {...(entry.external ? externalProps : {})}
                    onClick={(event) => {
                      if (!entry.onSelect || event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
                      event.preventDefault();
                      entry.onSelect();
                    }}
                  >
                    {Icon ? <Icon aria-hidden className="text-fg-subtle size-icon-16 shrink-0" /> : null}
                    <span className="min-w-0 flex-1 wrap-anywhere">{entry.label}</span>
                    {entry.shortcut ? <kbd className="text-fg-muted border-border bg-surface-sunken rounded-sm border px-1 font-mono text-2xs">{entry.shortcut}</kbd> : null}
                    {entry.external ? (
                      <>
                        <ExternalLink aria-hidden className="text-fg-subtle size-icon-16 shrink-0" />
                        <span className="sr-only">{intl.formatMessage(messages.opensInNewTab)}</span>
                      </>
                    ) : null}
                  </a>
                </MenuPrimitive.Item>
              );
            }
            const Icon = entry.icon;
            const Item = entry.selected === undefined ? MenuPrimitive.Item : MenuPrimitive.CheckboxItem;
            return (
              <Item
                key={entry.label}
                disabled={entry.disabled}
                onSelect={entry.onSelect}
                {...(entry.selected === undefined ? {} : { checked: entry.selected })}
                className={cn(menuItem, entry.destructive && 'text-status-danger')}
              >
                {entry.leading ?? (Icon ? <Icon aria-hidden className="text-fg-subtle size-icon-16 shrink-0" /> : null)}
                <span className="min-w-0 flex-1 wrap-anywhere">{entry.label}</span>
                {entry.shortcut ? <kbd className="text-fg-muted border-border bg-surface-sunken rounded-sm border px-1 font-mono text-2xs">{entry.shortcut}</kbd> : null}
                {entry.selected ? <Check aria-hidden className="text-fg-accent size-icon-16 shrink-0" /> : null}
              </Item>
            );
          })}
        </MenuPrimitive.Content>
      </MenuPrimitive.Portal>
    </MenuPrimitive.Root>
  );
}
