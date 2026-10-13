// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from 'vitest';

import { appearanceSearchSchema, changedPreferences, preferenceOfSection, preferencesSchema, sections, type Preferences } from './preferences';

const current: Preferences = { theme: 'system', density: 'comfortable', locale: 'en' };

describe('preferencesSchema', () => {
  it('accepts every valid combination', () => {
    expect(preferencesSchema.parse({ theme: 'dark', density: 'compact', locale: 'es' })).toEqual({ theme: 'dark', density: 'compact', locale: 'es' });
  });

  it('rejects a value that is not a choice', () => {
    expect(preferencesSchema.safeParse({ ...current, theme: 'sepia' }).success).toBe(false);
    expect(preferencesSchema.safeParse({ ...current, locale: 'xx-LS' }).success).toBe(false);
    expect(preferencesSchema.safeParse({ theme: 'light' }).success).toBe(false);
  });
});

describe('appearanceSearchSchema', () => {
  it.each(sections)('reads the section "%s"', (section) => {
    expect(appearanceSearchSchema.parse({ section })).toEqual({ section });
  });

  it('treats a missing or unknown section as no section', () => {
    expect(appearanceSearchSchema.parse({})).toEqual({ section: undefined });
    expect(appearanceSearchSchema.parse({ section: 'colors' })).toEqual({ section: undefined });
    expect(appearanceSearchSchema.parse({ section: 42 })).toEqual({ section: undefined });
  });
});

describe('preferenceOfSection', () => {
  it('maps each section to the preference it edits', () => {
    expect(preferenceOfSection('theme')).toBe('theme');
    expect(preferenceOfSection('density')).toBe('density');
    expect(preferenceOfSection('language')).toBe('locale');
  });
});

describe('changedPreferences', () => {
  it('returns nothing when nothing changed', () => {
    expect(changedPreferences(current, { ...current })).toEqual({});
  });

  it('returns only what changed', () => {
    expect(changedPreferences(current, { ...current, theme: 'dark' })).toEqual({ theme: 'dark' });
    expect(changedPreferences(current, { theme: 'light', density: 'compact', locale: 'es' })).toEqual({ theme: 'light', density: 'compact', locale: 'es' });
  });
});
