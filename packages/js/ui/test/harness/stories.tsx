// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { composeStories } from '@storybook/react-vite';
import type { ComponentType } from 'react';

import preview from '../../.storybook/preview';

type StoryModule = Parameters<typeof composeStories>[0];

export interface Globals {
  theme: 'light' | 'dark';
  density: 'comfortable' | 'compact';
  locale: 'en' | 'es' | 'xx-LS';
}

export interface ComposedEntry {
  /** "Components/Button › Hover" */
  label: string;
  title: string;
  name: string;
  Story: ComponentType<Record<string, unknown>> & {
    play?: (context: { canvasElement: HTMLElement }) => Promise<void> | void;
    parameters?: Record<string, unknown>;
  };
}

const modules = import.meta.glob<StoryModule>('../../src/**/*.stories.tsx', { eager: true });

/** Composes every story in the package with the preview decorators and the given globals. */
export function composeAll(globals: Partial<Globals>): ComposedEntry[] {
  const annotations = {
    ...preview,
    initialGlobals: { ...preview.initialGlobals, ...globals },
  };
  const entries: ComposedEntry[] = [];
  for (const [path, module] of Object.entries(modules)) {
    const meta = (module as { default?: { title?: string } }).default;
    const title = meta?.title ?? path;
    const composed = composeStories(module, annotations) as Record<string, ComposedEntry['Story']>;
    for (const [name, Story] of Object.entries(composed)) {
      entries.push({ label: `${title} › ${name}`, title, name, Story });
    }
  }
  return entries;
}
