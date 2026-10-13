// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Ref, TextareaHTMLAttributes } from 'react';

import { cn } from '../../lib/cn';
import { textControl } from '../../lib/control';
import { useFieldControlProps } from '../form-field/field-context';

export interface TextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  ref?: Ref<HTMLTextAreaElement> | undefined;
}

/** Multi-line text input. Grows vertically only; it never clears what a person typed. */
export function Textarea({ className, rows = 4, ref, ...rest }: TextareaProps) {
  const props = useFieldControlProps(rest);
  return <textarea ref={ref} rows={rows} className={cn(textControl, 'resize-y py-2', className)} {...props} />;
}
