// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { LucideIcon } from 'lucide-react';
import type { InputHTMLAttributes, ReactNode, Ref } from 'react';

import { cn } from '../../lib/cn';
import { textControl } from '../../lib/control';
import { useFieldControlProps } from '../form-field/field-context';

export interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  ref?: Ref<HTMLInputElement> | undefined;
  /** An icon inside the field, before the text. Decorative: the label says what the field is. */
  startIcon?: LucideIcon | undefined;
  /** A control inside the field, after the text, such as the button that shows a password. It is its own tab stop. */
  endAdornment?: ReactNode | undefined;
}

/** Single-line text input. Inside a FormField it is labeled, described and marked invalid automatically. */
export function Input({ className, type = 'text', startIcon: StartIcon, endAdornment, ref, ...rest }: InputProps) {
  const props = useFieldControlProps(rest);
  if (StartIcon === undefined && endAdornment === undefined) {
    return <input ref={ref} type={type} className={cn(textControl, 'h-control', className)} {...props} />;
  }
  return (
    <div className="relative flex items-center">
      {StartIcon ? <StartIcon aria-hidden className="text-fg-subtle pointer-events-none absolute left-3 size-icon-16" /> : null}
      <input ref={ref} type={type} className={cn(textControl, 'h-control', StartIcon && 'pl-10', endAdornment && 'pr-12', className)} {...props} />
      {endAdornment ? <span className="absolute right-1 flex items-center">{endAdornment}</span> : null}
    </div>
  );
}
