// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createContext, useContext, type AriaAttributes } from 'react';

export interface FieldContextValue {
  /** Id of the control; the label points at it. */
  controlId: string;
  /** Id of the label (or legend, for groups). */
  labelId: string;
  descriptionId?: string | undefined;
  errorId?: string | undefined;
  invalid: boolean;
  required: boolean;
}

export const FieldContext = createContext<FieldContextValue | null>(null);

export interface ControlOwnProps {
  id?: string | undefined;
  'aria-describedby'?: string | undefined;
  'aria-invalid'?: AriaAttributes['aria-invalid'] | undefined;
  'aria-required'?: AriaAttributes['aria-required'] | undefined;
}

/**
 * Merges the props a form control needs from its FormField (id, description
 * and error links, invalid and required) with the props passed directly. Props
 * passed directly win. Outside a FormField it returns them unchanged.
 */
export function useFieldControlProps<T extends ControlOwnProps>(props: T): T {
  const field = useContext(FieldContext);
  if (!field) return props;
  const describedBy = [props['aria-describedby'], field.descriptionId, field.errorId].filter(Boolean).join(' ');
  return {
    ...props,
    id: props.id ?? field.controlId,
    'aria-describedby': describedBy || undefined,
    'aria-invalid': props['aria-invalid'] ?? (field.invalid ? true : undefined),
    'aria-required': props['aria-required'] ?? (field.required ? true : undefined),
  };
}

export function useField(): FieldContextValue | null {
  return useContext(FieldContext);
}
