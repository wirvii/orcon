// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from 'vitest';

import { hasDisplayName, identityOf } from './identity';

describe('identityOf', () => {
  it('uses the account name when it is set', () => {
    expect(identityOf({ displayName: 'Ana Ruiz', email: 'x@example.com' })).toEqual({ name: 'Ana Ruiz', initials: 'AR' });
    expect(identityOf({ displayName: 'Madonna', email: 'x@example.com' })).toEqual({ name: 'Madonna', initials: 'MA' });
  });

  it('treats an empty name and the placeholder "Owner" as unset', () => {
    expect(identityOf({ displayName: 'Owner', email: 'jftamayop@example.com' })).toEqual({ name: 'jftamayop', initials: 'JF' });
    expect(identityOf({ displayName: ' owner ', email: 'jftamayop@example.com' }).name).toBe('jftamayop');
    expect(identityOf({ displayName: '', email: 'jftamayop@example.com' }).initials).toBe('JF');
  });

  it('reads separators in the part before the @ as words', () => {
    expect(identityOf({ displayName: 'Owner', email: 'ana.ruiz@example.com' })).toEqual({ name: 'Ana Ruiz', initials: 'AR' });
    expect(identityOf({ displayName: '', email: 'ana_maria-ruiz+ops@example.com' })).toEqual({ name: 'Ana Maria Ruiz Ops', initials: 'AM' });
  });

  it('never shows "Owner", even for an odd email', () => {
    expect(identityOf({ displayName: 'Owner', email: '@example.com' }).name).not.toBe('Owner');
  });
});

describe('hasDisplayName', () => {
  it('is false for an empty name and for the placeholder "Owner"', () => {
    expect(hasDisplayName({ displayName: 'Ana Ruiz' })).toBe(true);
    expect(hasDisplayName({ displayName: 'Owner' })).toBe(false);
    expect(hasDisplayName({ displayName: '  ' })).toBe(false);
  });
});
