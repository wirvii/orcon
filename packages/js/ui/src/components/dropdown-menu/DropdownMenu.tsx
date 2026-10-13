// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { DropdownMenu as MenuPrimitive } from 'radix-ui';
import type { ComponentProps, ReactElement } from 'react';

import { cn } from '../../lib/cn';
import { floatingPanel, menuItem } from '../../lib/control';

export type MenuEntry =
  | {
      type: 'item';
      label: string;
      onSelect?: () => void;
      disabled?: boolean;
      /** Irreversible or risky action. Shown in the danger color with the label unchanged; confirm it with an AlertDialog. */
      destructive?: boolean;
    }
  | { type: 'separator' }
  | { type: 'label'; label: string };

export interface DropdownMenuProps extends Pick<ComponentProps<typeof MenuPrimitive.Root>, 'open' | 'defaultOpen' | 'onOpenChange'> {
  /** The button that opens the menu. */
  trigger: ReactElement;
  items: readonly MenuEntry[];
  align?: ComponentProps<typeof MenuPrimitive.Content>['align'];
}

/** A list of actions opened from a button. Arrow keys move, Enter selects, Escape closes and returns focus. */
export function DropdownMenu({ trigger, items, align = 'end', ...root }: DropdownMenuProps) {
  return (
    // Non-modal: the rest of the page stays exposed, which keeps the trigger reachable for assistive technology.
    <MenuPrimitive.Root modal={false} {...root}>
      <MenuPrimitive.Trigger asChild>{trigger}</MenuPrimitive.Trigger>
      <MenuPrimitive.Portal>
        <MenuPrimitive.Content align={align} sideOffset={4} collisionPadding={8} className={cn(floatingPanel, 'min-w-trigger max-h-available overflow-y-auto p-1')}>
          {items.map((entry, index) => {
            if (entry.type === 'separator') {
              return <MenuPrimitive.Separator key={`separator-${index}`} className="border-border my-1 border-t" />;
            }
            if (entry.type === 'label') {
              return (
                <MenuPrimitive.Label key={`label-${entry.label}`} className="text-fg-muted px-control-x py-1 text-sm font-medium">
                  {entry.label}
                </MenuPrimitive.Label>
              );
            }
            return (
              <MenuPrimitive.Item
                key={entry.label}
                disabled={entry.disabled}
                onSelect={entry.onSelect}
                className={cn(menuItem, entry.destructive && 'text-status-danger')}
              >
                {entry.label}
              </MenuPrimitive.Item>
            );
          })}
        </MenuPrimitive.Content>
      </MenuPrimitive.Portal>
    </MenuPrimitive.Root>
  );
}
