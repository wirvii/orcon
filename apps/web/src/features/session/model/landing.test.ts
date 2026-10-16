// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from 'vitest';

import { landingFor } from './landing';

describe('landingFor', () => {
  it('no organization', () => {
    expect(landingFor([])).toEqual({ kind: 'none' });
  });

  it('one organization', () => {
    expect(landingFor([{ slug: 'acme' }])).toEqual({ kind: 'organization', slug: 'acme' });
  });

  it('several organizations', () => {
    expect(landingFor([{ slug: 'acme' }, { slug: 'globex' }])).toEqual({ kind: 'organizations' });
  });

  it('keeps a safe redirect', () => {
    expect(landingFor([{ slug: 'acme' }, { slug: 'globex' }], '/orgs/acme/projects')).toEqual({ kind: 'redirect', href: '/orgs/acme/projects' });
    expect(landingFor([], 'https://evil.example')).toEqual({ kind: 'none' });
  });
});
