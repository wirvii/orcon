// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createIntl } from 'react-intl';
import { describe, expect, it } from 'vitest';
import { z } from 'zod';

import en from '../../i18n/en.json';
import es from '../../i18n/es.json';

import { formatIssue, zodIssueMessage } from './zod-messages';

const english = createIntl({ locale: 'en', messages: en });
const spanish = createIntl({ locale: 'es', messages: es });

/** The first issue Zod reports for a value that breaks the schema. */
function issueOf(schema: z.ZodType, value: unknown): z.core.$ZodIssue {
  const issue = schema.safeParse(value).error?.issues[0];
  if (!issue) throw new Error('The value was expected to break the schema.');
  return issue;
}

describe('zodIssueMessage', () => {
  it.each([
    ['invalid_type', z.string(), 3, 'validation.invalid_type'],
    ['too_small', z.string().min(3), 'ab', 'validation.too_small'],
    ['too_big', z.string().max(2), 'abc', 'validation.too_big'],
    ['invalid_format', z.email(), 'nope', 'validation.invalid_format'],
    ['invalid_value', z.enum(['a', 'b']), 'c', 'validation.invalid_value'],
    ['invalid_union', z.union([z.string(), z.number()]), true, 'validation.invalid_union'],
    ['not_multiple_of', z.number().multipleOf(5), 7, 'validation.not_multiple_of'],
    ['unrecognized_keys', z.strictObject({ a: z.string() }), { a: 'x', b: 1 }, 'validation.unrecognized_keys'],
    ['custom', z.string().refine(() => false), 'x', 'validation.custom'],
  ])('maps %s to its catalog message', (_code, schema, value, id) => {
    expect(zodIssueMessage(issueOf(schema, value)).descriptor.id).toBe(id);
  });

  it('treats a text that must have at least one character as a required field', () => {
    expect(zodIssueMessage(issueOf(z.string().min(1), '')).descriptor.id).toBe('validation.required');
  });

  it('carries the numbers and names a message mentions', () => {
    expect(zodIssueMessage(issueOf(z.string().min(3), 'ab')).values).toEqual({ origin: 'string', min: 3 });
    expect(zodIssueMessage(issueOf(z.number().max(10), 11)).values).toEqual({ origin: 'number', max: 10 });
    expect(zodIssueMessage(issueOf(z.email(), 'x')).values).toEqual({ format: 'email' });
    expect(zodIssueMessage(issueOf(z.number().multipleOf(5), 7)).values).toEqual({ step: 5 });
  });

  it('reads the bounds of a bigint rule as numbers', () => {
    expect(zodIssueMessage(issueOf(z.bigint().min(5n), 1n)).values).toEqual({ origin: 'bigint', min: 5 });
  });
});

describe('formatIssue', () => {
  it('writes English messages from the catalog, never the raw Zod message', () => {
    expect(formatIssue(english, issueOf(z.string().min(1), ''))).toBe('This field is required.');
    expect(formatIssue(english, issueOf(z.string().min(3), 'ab'))).toBe('Use at least 3 characters.');
    expect(formatIssue(english, issueOf(z.string().min(1).max(1), 'ab'))).toBe('Use at most 1 character.');
    expect(formatIssue(english, issueOf(z.array(z.string()).min(2), ['a']))).toBe('Choose at least 2 items.');
    expect(formatIssue(english, issueOf(z.number().min(18), 10))).toBe('Must be at least 18.');
    expect(formatIssue(english, issueOf(z.url(), 'nope'))).toBe('Enter a valid web address.');
    expect(formatIssue(english, issueOf(z.string().regex(/^a$/), 'b'))).toBe('The format of this value is not valid.');
  });

  it('writes the same messages in Spanish', () => {
    expect(formatIssue(spanish, issueOf(z.string().min(1), ''))).toBe('Este campo es obligatorio.');
    expect(formatIssue(spanish, issueOf(z.string().min(3), 'ab'))).toBe('Usa al menos 3 caracteres.');
    expect(formatIssue(spanish, issueOf(z.array(z.string()).max(1), ['a', 'b']))).toBe('Elige como máximo 1 elemento.');
    expect(formatIssue(spanish, issueOf(z.enum(['a']), 'z'))).toBe('Elige una de las opciones disponibles.');
  });

  it('accepts an issue as a schema error map receives it', () => {
    const schema = z.object({ name: z.string().min(1) });
    const result = schema.safeParse({ name: '' }, { error: (issue) => formatIssue(english, issue) });
    expect(result.error?.issues[0]?.message).toBe('This field is required.');
  });
});
