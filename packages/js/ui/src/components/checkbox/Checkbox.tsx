// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Check, Minus } from 'lucide-react';
import { Checkbox as CheckboxPrimitive } from 'radix-ui';
import type { ComponentProps, ReactNode } from 'react';

import { choiceBox, choiceRow } from '../../lib/choice';
import { cn } from '../../lib/cn';
import { useFieldControlProps } from '../form-field/field-context';

export interface CheckboxProps extends Omit<ComponentProps<typeof CheckboxPrimitive.Root>, 'children'> {
  /** The visible label. It is inside the control, so the whole row is clickable. */
  label: ReactNode;
}

export function Checkbox({ label, className, ...rest }: CheckboxProps) {
  const props = useFieldControlProps(rest);
  return (
    <CheckboxPrimitive.Root className={cn(choiceRow, className)} {...props}>
      <span
        className={cn(
          choiceBox,
          'rounded-sm group-data-[state=checked]:border-action-primary group-data-[state=checked]:bg-action-primary',
          'group-data-[state=indeterminate]:border-action-primary group-data-[state=indeterminate]:bg-action-primary',
        )}
      >
        <CheckboxPrimitive.Indicator>
          <Check aria-hidden className="size-icon-16 group-data-[state=indeterminate]:hidden" />
          <Minus aria-hidden className="size-icon-16 hidden group-data-[state=indeterminate]:block" />
        </CheckboxPrimitive.Indicator>
      </span>
      <span className="min-w-0 wrap-break-word">{label}</span>
    </CheckboxPrimitive.Root>
  );
}
