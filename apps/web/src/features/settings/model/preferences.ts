// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { z } from 'zod';

/** The choices of the appearance settings. They mirror the design system's providers. */
export const themeChoices = ['system', 'light', 'dark'] as const;
export const densityChoices = ['comfortable', 'compact'] as const;
export const localeChoices = ['en', 'es'] as const;

/** The preferences a person can change. */
export const preferencesSchema = z.object({
  theme: z.enum(themeChoices),
  density: z.enum(densityChoices),
  locale: z.enum(localeChoices),
});

export type Preferences = z.infer<typeof preferencesSchema>;

/** The sections of the appearance page. The URL names one (`?section=language`). */
export const sections = ['theme', 'density', 'language'] as const;

export type Section = (typeof sections)[number];

/**
 * The search parameters of the appearance page. A missing or unknown `section` is simply
 * no section: a malformed link falls back instead of failing (orcon-web RT-3).
 */
export const appearanceSearchSchema = z.object({
  section: z.enum(sections).optional().catch(undefined),
});

export type AppearanceSearch = z.infer<typeof appearanceSearchSchema>;

/** The preference each section edits. */
export function preferenceOfSection(section: Section): keyof Preferences {
  return section === 'language' ? 'locale' : section;
}

/** The preferences in `next` that differ from `current`; nothing else is applied or saved. */
export function changedPreferences(current: Preferences, next: Preferences): Partial<Preferences> {
  const changed: Partial<Preferences> = {};
  if (current.theme !== next.theme) changed.theme = next.theme;
  if (current.density !== next.density) changed.density = next.density;
  if (current.locale !== next.locale) changed.locale = next.locale;
  return changed;
}
