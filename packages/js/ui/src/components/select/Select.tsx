// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Check, ChevronDown } from 'lucide-react';
import { Select as SelectPrimitive } from 'radix-ui';
import type { AriaAttributes, ComponentProps } from 'react';

import { cn } from '../../lib/cn';
import { floatingPanel, menuItem, textControl } from '../../lib/control';
import { useFieldControlProps } from '../form-field/field-context';

export interface SelectOption {
  value: string;
  label: string;
  disabled?: boolean;
}

export interface SelectProps extends Omit<ComponentProps<typeof SelectPrimitive.Root>, 'children'> {
  options: readonly SelectOption[];
  /** Text shown when nothing is selected. Pass a translated string. */
  placeholder?: string;
  id?: string;
  'aria-label'?: string;
  'aria-describedby'?: string;
  'aria-invalid'?: AriaAttributes['aria-invalid'];
  className?: string;
}

/** Single choice from a list. Inside a FormField it is labeled, described and marked invalid automatically. */
export function Select({ options, placeholder, className, id, 'aria-label': ariaLabel, 'aria-describedby': describedBy, 'aria-invalid': invalid, ...root }: SelectProps) {
  const control = useFieldControlProps({ id, 'aria-describedby': describedBy, 'aria-invalid': invalid });
  return (
    <SelectPrimitive.Root {...root}>
      <SelectPrimitive.Trigger
        {...control}
        aria-label={ariaLabel}
        className={cn(
          textControl,
          'h-control flex items-center justify-between gap-2 text-left data-[placeholder]:text-fg-subtle',
          className,
        )}
      >
        <SelectPrimitive.Value placeholder={placeholder} />
        <SelectPrimitive.Icon>
          <ChevronDown aria-hidden className="size-icon-16 text-fg-muted" />
        </SelectPrimitive.Icon>
      </SelectPrimitive.Trigger>
      <SelectPrimitive.Portal>
        <SelectPrimitive.Content position="popper" sideOffset={4} className={cn(floatingPanel, 'min-w-trigger max-h-available overflow-hidden')}>
          <SelectPrimitive.Viewport className="p-1">
            {options.map((option) => (
              <SelectPrimitive.Item key={option.value} value={option.value} disabled={option.disabled} className={menuItem}>
                <SelectPrimitive.ItemText>{option.label}</SelectPrimitive.ItemText>
                <SelectPrimitive.ItemIndicator className="ml-auto">
                  <Check aria-hidden className="size-icon-16" />
                </SelectPrimitive.ItemIndicator>
              </SelectPrimitive.Item>
            ))}
          </SelectPrimitive.Viewport>
        </SelectPrimitive.Content>
      </SelectPrimitive.Portal>
    </SelectPrimitive.Root>
  );
}
