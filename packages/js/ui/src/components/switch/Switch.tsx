// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Switch as SwitchPrimitive } from 'radix-ui';
import type { ComponentProps, ReactNode } from 'react';

import { choiceRow } from '../../lib/choice';
import { cn } from '../../lib/cn';
import { useFieldControlProps } from '../form-field/field-context';

export interface SwitchProps extends Omit<ComponentProps<typeof SwitchPrimitive.Root>, 'children'> {
  /** What the switch turns on. State is carried by the switch role and the thumb position, not by color. */
  label: ReactNode;
}

export function Switch({ label, className, ...rest }: SwitchProps) {
  const props = useFieldControlProps(rest);
  return (
    <SwitchPrimitive.Root className={cn(choiceRow, className)} {...props}>
      <span
        className={cn(
          'border-control bg-surface-sunken flex h-icon-20 w-8 shrink-0 items-center rounded-full border px-1 transition-colors',
          'group-data-[state=checked]:border-action-primary group-data-[state=checked]:bg-action-primary',
          'group-disabled:border-border',
        )}
      >
        <SwitchPrimitive.Thumb
          className={cn(
            'bg-fg-muted size-3 rounded-full transition-transform',
            'data-[state=checked]:bg-action-primary-fg data-[state=checked]:translate-x-3',
            'group-disabled:bg-fg-disabled',
          )}
        />
      </span>
      <span>{label}</span>
    </SwitchPrimitive.Root>
  );
}
