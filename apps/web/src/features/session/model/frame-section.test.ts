// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from 'vitest';

import { frameSectionOf, sidebarIdOf, tabIdOf } from './frame-section';

describe('frameSectionOf', () => {
  it.each([
    ['/orgs/acme/overview', { kind: 'home' }],
    ['/orgs/acme/projects', { kind: 'projects' }],
    ['/orgs/acme/projects/sales/flows', { kind: 'project', handle: 'sales' }],
    ['/orgs/acme/projects/sales/credentials', { kind: 'project', handle: 'sales' }],
    ['/orgs/acme/projects/sales/flows/0195f1a0-0000-7000-8000-000000000001', { kind: 'project', handle: 'sales' }],
    ['/orgs/acme/templates', { kind: 'templates' }],
    ['/orgs/acme/credentials', { kind: 'credentials' }],
    ['/orgs/acme/executions', { kind: 'executions' }],
    ['/orgs/acme/settings/appearance', { kind: 'settings' }],
    ['/orgs/acme', { kind: 'other' }],
    ['/orgs/other/overview', { kind: 'other' }],
    ['/orgs', { kind: 'other' }],
  ])('reads %s', (pathname, expected) => {
    expect(frameSectionOf(pathname, 'acme')).toEqual(expected);
  });

  it('does not mistake an organization whose name starts with the same letters', () => {
    expect(frameSectionOf('/orgs/acme-2/overview', 'acme')).toEqual({ kind: 'other' });
  });
});

describe('the marked entries', () => {
  it('marks only the sidebar entry that owns the address', () => {
    expect(sidebarIdOf({ kind: 'home' })).toBe('home');
    expect(sidebarIdOf({ kind: 'project', handle: 'sales' })).toBe('project:sales');
    // The list of all projects has no entry, so nothing is marked there.
    expect(sidebarIdOf({ kind: 'projects' })).toBeUndefined();
    expect(sidebarIdOf({ kind: 'other' })).toBeUndefined();
  });

  it('marks a tab of the phone bar for each part that has one', () => {
    expect(tabIdOf({ kind: 'projects' })).toBe('flows');
    expect(tabIdOf({ kind: 'project', handle: 'sales' })).toBe('flows');
    expect(tabIdOf({ kind: 'settings' })).toBe('settings');
    expect(tabIdOf({ kind: 'templates' })).toBeUndefined();
  });
});
