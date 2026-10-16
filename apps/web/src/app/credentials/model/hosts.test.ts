// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from 'vitest';

import { parseAllowedHost, withHost } from './hosts';

describe('parseAllowedHost', () => {
  it.each([
    ['api.siigo.com', 'api.siigo.com'],
    ['  API.Siigo.COM. ', 'api.siigo.com'],
    ['*.example.com', '*.example.com'],
    ['https://api.siigo.com/v1/invoices?x=1', 'api.siigo.com'],
    ['api.siigo.com:8443/path', 'api.siigo.com'],
    ['10.0.0.7', '10.0.0.7'],
  ])('reads %j as the host %j', (input, host) => {
    expect(parseAllowedHost(input)).toEqual({ ok: true, host });
  });

  it.each([
    ['', 'empty'],
    ['   ', 'empty'],
    ['localhost', 'invalid'],
    ['-bad.example.com', 'invalid'],
    ['exa mple.com', 'invalid'],
    ['a..example.com', 'invalid'],
    ['[::1]', 'invalid'],
    ['*.10.0.0.7', 'wildcard-address'],
    ['*.1.2.3', 'wildcard-address'],
  ])('refuses %j because it is %s', (input, reason) => {
    expect(parseAllowedHost(input)).toEqual({ ok: false, reason });
  });

  it('refuses a name longer than 253 characters', () => {
    expect(parseAllowedHost(`${'a'.repeat(63)}.${'b'.repeat(63)}.${'c'.repeat(63)}.${'d'.repeat(63)}.com`).ok).toBe(false);
  });
});

describe('withHost', () => {
  it('does not repeat a host', () => {
    expect(withHost(['a.example.com'], 'a.example.com')).toEqual(['a.example.com']);
    expect(withHost(['a.example.com'], 'b.example.com')).toEqual(['a.example.com', 'b.example.com']);
  });
});
