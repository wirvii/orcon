// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { ChevronDown, type LucideIcon } from 'lucide-react';
import type { ButtonHTMLAttributes, Ref } from 'react';

import { cn } from '../../lib/cn';

export interface FilterChipProps extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'children'> {
  ref?: Ref<HTMLButtonElement> | undefined;
  /** What is filtered: "Status", "Project". */
  label: string;
  /** The choice made, such as "all" or "Failed". Shown after the label. */
  value?: string | undefined;
  icon?: LucideIcon | undefined;
  /** A filter that is narrowing the list right now. It gets a stronger outline and a pressed state. */
  pressed?: boolean | undefined;
  /** Show the chevron of a chip that opens a list of choices. */
  menu?: boolean | undefined;
}

/**
 * A filter shown as a small outlined button: icon, what is filtered and the choice made. It carries the
 * pressed state in `aria-pressed` and in a stronger outline, never in color alone.
 */
export function FilterChip({ label, value, icon: Icon, pressed = false, menu = false, className, ref, type = 'button', ...props }: FilterChipProps) {
  return (
    <button
      ref={ref}
      type={type}
      aria-pressed={menu ? undefined : pressed}
      className={cn(
        'border-strong text-fg-muted inline-flex min-h-target max-w-full items-center gap-1 rounded-md border px-control-x py-1 text-xs font-medium whitespace-nowrap transition-colors',
        'hover:text-fg',
        pressed && 'border-control bg-surface-hover text-fg',
        className,
      )}
      {...props}
    >
      {Icon ? <Icon aria-hidden className="size-icon-16 shrink-0" /> : null}
      <span>{value === undefined ? label : `${label}: ${value}`}</span>
      {menu ? <ChevronDown aria-hidden className="size-icon-16 shrink-0" /> : null}
    </button>
  );
}
