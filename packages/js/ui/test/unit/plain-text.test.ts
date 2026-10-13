// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from 'vitest';

import { plainText } from '../../src/editor/model';

describe('plainText', () => {
  it('prints a string without quotes, as the data views always did', () => {
    expect(plainText('ada')).toBe('ada');
  });

  it('prints numbers, booleans, bigints and symbols as themselves', () => {
    expect(plainText(42)).toBe('42');
    expect(plainText(false)).toBe('false');
    expect(plainText(10n)).toBe('10');
    expect(plainText(Symbol('k'))).toBe('Symbol(k)');
  });

  it('prints undefined as nothing', () => {
    expect(plainText(undefined)).toBe('');
  });

  it('prints an object as JSON', () => {
    expect(plainText({ a: 1 })).toBe('{"a":1}');
  });

  it('prints a function as nothing', () => {
    expect(plainText(() => 1)).toBe('');
  });
});
