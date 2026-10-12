// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

export type Theme = 'light' | 'dark';
export type Density = 'comfortable' | 'compact';

export interface Viewport {
  name: 'mobile' | 'tablet' | 'desktop';
  width: number;
  height: number;
}

export const viewports: Record<Viewport['name'], Viewport> = {
  mobile: { name: 'mobile', width: 390, height: 844 },
  tablet: { name: 'tablet', width: 1024, height: 768 },
  desktop: { name: 'desktop', width: 1440, height: 900 },
};

export const themes: readonly Theme[] = ['light', 'dark'];
export const densities: readonly Density[] = ['comfortable', 'compact'];

export interface Target {
  /** Storybook story id, such as "components-button--matrix". */
  id: string;
  title: string;
  kind: 'matrix' | 'screen';
  /** The viewports it is checked on. Every target uses mobile and desktop; the flow editor adds tablet. */
  viewports: Viewport[];
}

const here = dirname(fileURLToPath(import.meta.url));

/**
 * What the visual suite covers: the Matrix story of every component and canvas primitive, and every
 * screen template (its Matrix story is the template in its default state). Read from the built
 * Storybook index, so the suite follows the stories and never a hand-kept list.
 */
export function loadTargets(): Target[] {
  const index = JSON.parse(readFileSync(resolve(here, '../storybook-static/index.json'), 'utf8')) as {
    entries: Record<string, { id: string; type: string; title: string; name: string }>;
  };
  const targets: Target[] = [];
  for (const entry of Object.values(index.entries)) {
    if (entry.type !== 'story' || entry.name !== 'Matrix') continue;
    const screen = entry.title.startsWith('Screens/');
    const editor = entry.title.startsWith('Screens/S3 ');
    targets.push({
      id: entry.id,
      title: entry.title,
      kind: screen ? 'screen' : 'matrix',
      viewports: editor ? [viewports.mobile, viewports.tablet, viewports.desktop] : [viewports.mobile, viewports.desktop],
    });
  }
  return targets.sort((a, b) => a.id.localeCompare(b.id));
}

export function storyUrl(id: string, globals: { theme: Theme; density: Density; locale?: string }): string {
  const parts = [`theme:${globals.theme}`, `density:${globals.density}`, `locale:${globals.locale ?? 'en'}`];
  return `/iframe.html?id=${encodeURIComponent(id)}&viewMode=story&globals=${encodeURIComponent(parts.join(';'))}`;
}
