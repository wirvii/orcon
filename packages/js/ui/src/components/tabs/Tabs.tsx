// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Tabs as TabsPrimitive } from 'radix-ui';
import type { ReactNode } from 'react';

import { cn } from '../../lib/cn';

export interface TabItem {
  value: string;
  label: string;
  content: ReactNode;
  disabled?: boolean;
  className?: string;
}

export interface TabsProps {
  tabs: readonly TabItem[];
  /** Names the tab list. */
  label: string;
  value?: string;
  defaultValue?: string;
  onValueChange?: (value: string) => void;
  className?: string;
}

/**
 * Switches between views of the same thing. Arrow keys move between tabs, Tab
 * moves into the panel. The selected tab is marked by a bar and a heavier weight
 * as well as by color.
 */
export function Tabs({ tabs, label, value, defaultValue, onValueChange, className }: TabsProps) {
  return (
    <TabsPrimitive.Root value={value} defaultValue={defaultValue ?? tabs[0]?.value} onValueChange={onValueChange} className={className}>
      <TabsPrimitive.List aria-label={label} className="border-border flex border-b">
        {tabs.map((tab) => (
          <TabsPrimitive.Trigger
            key={tab.value}
            value={tab.value}
            disabled={tab.disabled}
            className={cn(
              'text-fg-muted inline-flex min-h-target min-w-target items-center border-b-2 border-transparent px-control-x py-2 text-base wrap-anywhere whitespace-normal transition-colors',
              'hover:text-fg data-[state=active]:border-action-primary data-[state=active]:text-fg data-[state=active]:font-semibold',
              'disabled:pointer-events-none disabled:text-fg-disabled',
              tab.className,
            )}
          >
            {tab.label}
          </TabsPrimitive.Trigger>
        ))}
      </TabsPrimitive.List>
      {tabs.map((tab) => (
        <TabsPrimitive.Content key={tab.value} value={tab.value} className="pt-3">
          {tab.content}
        </TabsPrimitive.Content>
      ))}
    </TabsPrimitive.Root>
  );
}
