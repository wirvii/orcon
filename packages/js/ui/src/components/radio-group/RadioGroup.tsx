// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { RadioGroup as RadioGroupPrimitive } from 'radix-ui';
import type { ComponentProps, ReactNode } from 'react';

import { choiceBox, choiceRow } from '../../lib/choice';
import { cn } from '../../lib/cn';
import { useField } from '../form-field/field-context';

export type RadioGroupProps = ComponentProps<typeof RadioGroupPrimitive.Root>;

/**
 * A set of mutually exclusive options. Put it in a `<FormField group label="…">`
 * and it takes its name from the legend; otherwise pass aria-label.
 */
export function RadioGroup({ className, ...props }: RadioGroupProps) {
  const field = useField();
  return (
    <RadioGroupPrimitive.Root
      aria-labelledby={field?.labelId}
      aria-describedby={[field?.descriptionId, field?.errorId].filter(Boolean).join(' ') || undefined}
      aria-invalid={field?.invalid ? true : undefined}
      aria-required={field?.required ? true : undefined}
      className={cn('flex flex-col gap-1', className)}
      {...props}
    />
  );
}

export interface RadioGroupItemProps extends Omit<ComponentProps<typeof RadioGroupPrimitive.Item>, 'children'> {
  label: ReactNode;
}

export function RadioGroupItem({ label, className, ...props }: RadioGroupItemProps) {
  return (
    <RadioGroupPrimitive.Item className={cn(choiceRow, className)} {...props}>
      <span className={cn(choiceBox, 'rounded-full group-data-[state=checked]:border-action-primary')}>
        <RadioGroupPrimitive.Indicator className="bg-action-primary flex size-full items-center justify-center rounded-full">
          <span className="bg-action-primary-fg size-1 rounded-full" />
        </RadioGroupPrimitive.Indicator>
      </span>
      <span>{label}</span>
    </RadioGroupPrimitive.Item>
  );
}
