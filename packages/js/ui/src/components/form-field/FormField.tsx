// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { CircleAlert } from 'lucide-react';
import { useId, type ReactNode } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { cn } from '../../lib/cn';

import { fieldIdForName } from '../problem-alert/problem';

import { FieldContext, type FieldContextValue } from './field-context';
import { useFieldProblemError } from './problem-errors';

const messages = defineMessages({
  required: {
    id: 'common.field.required',
    defaultMessage: 'required',
    description: 'Screen reader text after a form field label that marks the field as required.',
  },
  error: {
    id: 'common.field.errorPrefix',
    defaultMessage: 'Error:',
    description: 'Screen reader text before a form field error message.',
  },
});

export interface FormFieldProps {
  label: string;
  /** Helper text shown under the label and linked to the control. */
  description?: ReactNode;
  /** Error text. When set the control is marked invalid and the text is linked to it. */
  error?: ReactNode;
  required?: boolean;
  /**
   * The field is a group (radio group, checkbox group): the label becomes a
   * legend and no single control id is needed.
   */
  group?: boolean;
  /**
   * Field name: a JSON pointer such as "/title" or a plain name. A server error
   * whose pointer equals the name appears on this field (see ProblemErrorsProvider),
   * and the control gets the id `fieldIdForName(name)` so an ErrorSummary can link to it.
   */
  name?: string;
  /** Fixed control id. Defaults to one built from `name`, else a generated one. */
  id?: string;
  className?: string;
  children: ReactNode;
}

/**
 * Label, description, control and error as one unit. Controls read the ids and
 * the invalid and required state from here, so a control placed inside needs no
 * extra wiring. A server error maps to a field by its `name` (see ProblemAlert).
 */
export function FormField({ label, description, error: errorProp, required = false, group = false, name, id, className, children }: FormFieldProps) {
  const intl = useIntl();
  const generated = useId();
  const problemError = useFieldProblemError(name);
  const error = errorProp ?? problemError;
  const controlId = id ?? (name ? fieldIdForName(name) : `field-${generated}`);
  const value: FieldContextValue = {
    controlId,
    labelId: `${controlId}-label`,
    descriptionId: description ? `${controlId}-description` : undefined,
    errorId: error ? `${controlId}-error` : undefined,
    invalid: Boolean(error),
    required,
  };

  const labelContent = (
    <>
      {label}
      {required ? (
        <>
          <span aria-hidden className="text-fg-muted ml-1">
            *
          </span>
          <span className="sr-only">({intl.formatMessage(messages.required)})</span>
        </>
      ) : null}
    </>
  );
  const labelClass = 'text-fg text-sm font-medium';

  const body = (
    <>
      {group ? (
        <legend id={value.labelId} className={cn(labelClass, 'mb-1')}>
          {labelContent}
        </legend>
      ) : (
        <label id={value.labelId} htmlFor={controlId} className={labelClass}>
          {labelContent}
        </label>
      )}
      {description ? (
        <p id={value.descriptionId} className="text-fg-muted text-sm">
          {description}
        </p>
      ) : null}
      {children}
      {error ? (
        <p id={value.errorId} className="text-status-danger flex items-start gap-1 text-sm">
          <CircleAlert aria-hidden className="size-icon-16 shrink-0" />
          <span>
            <span className="sr-only">{intl.formatMessage(messages.error)} </span>
            {error}
          </span>
        </p>
      ) : null}
    </>
  );

  return (
    <FieldContext.Provider value={value}>
      {group ? (
        <fieldset aria-describedby={[value.descriptionId, value.errorId].filter(Boolean).join(' ') || undefined} className={cn('flex min-w-0 flex-col gap-1 break-words', className)}>
          {body}
        </fieldset>
      ) : (
        <div className={cn('flex flex-col gap-1 break-words', className)}>{body}</div>
      )}
    </FieldContext.Provider>
  );
}
