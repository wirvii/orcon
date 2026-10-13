// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { afterEach, describe, expect, it, vi } from 'vitest';

import { EDITOR_READY_MARK, FIRST_USEFUL_MARK, markEditorReady, markFirstUsefulScreen } from './marks';

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  performance.clearMarks();
});

describe('marks', () => {
  it('uses the names the performance budgets measure', () => {
    expect(FIRST_USEFUL_MARK).toBe('orcon:first-useful');
    expect(EDITOR_READY_MARK).toBe('orcon:editor-ready');
  });

  it('marks the first useful screen with the route', () => {
    markFirstUsefulScreen('/settings/appearance');
    const [entry] = performance.getEntriesByName(FIRST_USEFUL_MARK);
    expect(entry?.entryType).toBe('mark');
    expect((entry as PerformanceMark).detail).toEqual({ route: '/settings/appearance' });
  });

  it('marks the editor ready', () => {
    markEditorReady();
    expect(performance.getEntriesByName(EDITOR_READY_MARK)).toHaveLength(1);
  });

  it('keeps every mark of a page load, so the first one can be read', () => {
    markFirstUsefulScreen('/');
    markFirstUsefulScreen('/settings/appearance');
    const routes = performance.getEntriesByName(FIRST_USEFUL_MARK).map((entry) => (entry as PerformanceMark).detail as { route: string });
    expect(routes.map((detail) => detail.route)).toEqual(['/', '/settings/appearance']);
  });

  it('does nothing where the User Timing API is missing', () => {
    vi.stubGlobal('performance', undefined);
    expect(() => {
      markFirstUsefulScreen('/');
      markEditorReady();
    }).not.toThrow();
    vi.stubGlobal('performance', {});
    expect(() => {
      markFirstUsefulScreen('/');
    }).not.toThrow();
  });
});
