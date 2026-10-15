// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from 'vitest';

import { safeRedirect } from './redirect';

describe('safeRedirect', () => {
  it('accepts a same-origin path', () => {
    expect(safeRedirect('/orgs/acme/projects?sort=name')).toBe('/orgs/acme/projects?sort=name');
  });

  it('rejects an absolute URL', () => {
    expect(safeRedirect('https://evil.example/phish')).toBeUndefined();
  });

  it('rejects a protocol-relative URL', () => {
    expect(safeRedirect('//evil.example/phish')).toBeUndefined();
  });

  it('rejects a backslash path', () => {
    expect(safeRedirect('/\\evil.example')).toBeUndefined();
  });

  it('rejects a javascript URL', () => {
    expect(safeRedirect('javascript:alert(1)')).toBeUndefined();
  });
});
