// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { clsx, type ClassValue } from 'clsx';
import { extendTailwindMerge } from 'tailwind-merge';

// The Orcon theme resets Tailwind's defaults, so tailwind-merge must learn the
// token-driven names that overlap with its built-in groups.
const twMerge = extendTailwindMerge({
  extend: {
    theme: {
      text: ['mono', 'md'],
      spacing: [
        'row',
        'control',
        'control-x',
        'density',
        'target',
        'panel-width',
        'canvas-node',
        'canvas-node-wide',
        'canvas-icon-box',
        'canvas-icon',
        'canvas-label-width',
        'canvas-label-gap',
        'canvas-ring-gap',
        'canvas-badge',
        'canvas-badge-glyph',
        'canvas-port-hover',
        'canvas-stub',
        'canvas-stub-box',
        'canvas-toolbar-button',
        'canvas-ai-block',
        'canvas-grid-step',
        'canvas-grid-dot',
        'canvas-minimap-w',
        'canvas-minimap-h',
        'canvas-palette-w',
        'editor-params-min',
        'editor-params-max',
        'editor-pane-min',
        'editor-docked-panel',
        'port',
        'connector-icon',
        'connector-icon-safe',
        'icon-16',
        'icon-20',
        'icon-24',
      ],
      shadow: ['sm', 'md', 'lg'],
      radius: ['sm', 'md', 'lg', 'xl', '2xl', 'full'],
    },
  },
});

/** Joins class names and resolves Tailwind conflicts (the last class wins). */
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}
