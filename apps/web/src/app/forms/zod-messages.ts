// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { defineMessages, type IntlShape, type MessageDescriptor } from 'react-intl';
import type { z } from 'zod';

/**
 * One catalog message per Zod issue code (orcon-web FM-4). A raw Zod message is never
 * shown: it is English only and written for developers.
 */
const messages = defineMessages({
  required: {
    id: 'validation.required',
    defaultMessage: 'This field is required.',
    description: 'Error under a form field that was left empty.',
  },
  invalid_type: {
    id: 'validation.invalid_type',
    defaultMessage: 'This value is not valid.',
    description: 'Error under a form field whose value is of the wrong kind.',
  },
  too_small: {
    id: 'validation.too_small',
    defaultMessage:
      '{origin, select, string {Use at least {min, plural, one {# character} other {# characters}}.} array {Choose at least {min, plural, one {# item} other {# items}}.} set {Choose at least {min, plural, one {# item} other {# items}}.} other {Must be at least {min}.}}',
    description: 'Error under a form field whose value is below the minimum. The origin says whether it is text, a list or a number.',
  },
  too_big: {
    id: 'validation.too_big',
    defaultMessage:
      '{origin, select, string {Use at most {max, plural, one {# character} other {# characters}}.} array {Choose at most {max, plural, one {# item} other {# items}}.} set {Choose at most {max, plural, one {# item} other {# items}}.} other {Must be at most {max}.}}',
    description: 'Error under a form field whose value is above the maximum. The origin says whether it is text, a list or a number.',
  },
  invalid_format: {
    id: 'validation.invalid_format',
    defaultMessage:
      '{format, select, email {Enter a valid email address.} url {Enter a valid web address.} uuid {Enter a valid identifier.} other {The format of this value is not valid.}}',
    description: 'Error under a form field whose text does not have the expected format, such as an email address.',
  },
  invalid_value: {
    id: 'validation.invalid_value',
    defaultMessage: 'Choose one of the available options.',
    description: 'Error under a form field whose value is not one of the allowed choices.',
  },
  invalid_union: {
    id: 'validation.invalid_union',
    defaultMessage: 'This value is not valid.',
    description: 'Error under a form field whose value matches none of the accepted shapes.',
  },
  not_multiple_of: {
    id: 'validation.not_multiple_of',
    defaultMessage: 'Must be a multiple of {step}.',
    description: 'Error under a number field whose value is not a multiple of the allowed step.',
  },
  unrecognized_keys: {
    id: 'validation.unrecognized_keys',
    defaultMessage: 'Some of the values are not recognized.',
    description: 'Error for a form that sent fields the schema does not know.',
  },
  custom: {
    id: 'validation.custom',
    defaultMessage: 'This value is not valid.',
    description: 'Error under a form field that fails a rule specific to the screen.',
  },
});

/** A catalog message and the values its placeholders need. */
export interface IssueMessage {
  readonly descriptor: MessageDescriptor;
  readonly values: Readonly<Record<string, string | number>>;
}

/**
 * A Zod issue as a resolver reports it (`$ZodIssue`) or as a schema's error map receives it
 * while parsing (`$ZodRawIssue`): both carry the same code and the same rule details.
 */
export type ZodIssueLike = z.core.$ZodIssue | z.core.$ZodRawIssue;

const asNumber = (bound: number | bigint): number => Number(bound);

/** The catalog message for a Zod issue, with the numbers and names it mentions. */
export function zodIssueMessage(issue: ZodIssueLike): IssueMessage {
  switch (issue.code) {
    case 'invalid_type':
      return { descriptor: messages.invalid_type, values: {} };
    case 'too_small':
      // A text that must have at least one character is a required field.
      if (issue.origin === 'string' && asNumber(issue.minimum) === 1) return { descriptor: messages.required, values: {} };
      return { descriptor: messages.too_small, values: { origin: issue.origin, min: asNumber(issue.minimum) } };
    case 'too_big':
      return { descriptor: messages.too_big, values: { origin: issue.origin, max: asNumber(issue.maximum) } };
    case 'invalid_format':
      return { descriptor: messages.invalid_format, values: { format: issue.format } };
    case 'invalid_value':
      return { descriptor: messages.invalid_value, values: {} };
    case 'invalid_union':
      return { descriptor: messages.invalid_union, values: {} };
    case 'not_multiple_of':
      return { descriptor: messages.not_multiple_of, values: { step: asNumber(issue.divisor) } };
    case 'unrecognized_keys':
      return { descriptor: messages.unrecognized_keys, values: {} };
    default:
      return { descriptor: messages.custom, values: {} };
  }
}

/** The translated text of a Zod issue for the active locale. */
export function formatIssue(intl: IntlShape, issue: ZodIssueLike): string {
  const { descriptor, values } = zodIssueMessage(issue);
  return intl.formatMessage(descriptor, values);
}
