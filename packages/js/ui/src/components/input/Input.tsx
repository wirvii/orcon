// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { InputHTMLAttributes, Ref } from 'react';

import { cn } from '../../lib/cn';
import { textControl } from '../../lib/control';
import { useFieldControlProps } from '../form-field/field-context';

export interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  ref?: Ref<HTMLInputElement>;
}

/** Single-line text input. Inside a FormField it is labeled, described and marked invalid automatically. */
export function Input({ className, type = 'text', ref, ...rest }: InputProps) {
  const props = useFieldControlProps(rest);
  return <input ref={ref} type={type} className={cn(textControl, 'h-control', className)} {...props} />;
}
