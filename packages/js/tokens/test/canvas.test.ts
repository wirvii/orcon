// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from 'vitest';

import { emitCss, emitTailwindTheme } from '../scripts/emit.mjs';

import { model } from './helpers/model';

const css = emitCss(model);

describe('canvas and editor tokens (style A)', () => {
  it('exposes the geometry of the node tile, ports, labels and grid', () => {
    for (const name of [
      'canvas-node-size',
      'canvas-node-wide',
      'canvas-icon-box',
      'canvas-icon',
      'canvas-label-width',
      'canvas-badge',
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
      'radius-2xl',
    ]) {
      expect(css, name).toContain(`--orcon-${name}:`);
    }
  });

  it('keeps the measures of the specification', () => {
    const scale = Object.fromEntries(model.scales.map((t) => [t.key, t.css]));
    expect(scale['canvas.node-size']).toBe('100px');
    expect(scale['canvas.grid-step']).toBe('20px');
    expect(scale['radius.2xl']).toBe('18px');
    expect(scale['canvas.lod.detail']).toBe('0.6');
    expect(scale['canvas.lod.minimal']).toBe('0.35');
    expect(scale['canvas.zoom.min']).toBe('0.2');
    expect(scale['canvas.zoom.max']).toBe('4');
  });

  it('defines the node background, the marquee and six sticky note colors in both themes', () => {
    for (const theme of ['light', 'dark'] as const) {
      const keys = new Set(model.themes[theme].map((t) => t.key));
      expect(keys.has('canvas.node.bg')).toBe(true);
      expect(keys.has('canvas.marquee.bg')).toBe(true);
      for (let n = 1; n <= 6; n += 1) {
        for (const part of ['bg', 'border', 'accent']) expect(keys.has(`canvas.sticky.${n}.${part}`), `${theme} sticky ${n} ${part}`).toBe(true);
      }
    }
  });

  it('never uses red, amber or violet for sticky notes', () => {
    const forbidden = /(red|amber|violet)/;
    for (const theme of ['light', 'dark'] as const) {
      for (const t of model.themes[theme].filter((x) => x.key.startsWith('canvas.sticky.'))) {
        expect(t.reference ?? '', `${theme} ${t.key}`).not.toMatch(forbidden);
      }
    }
  });

  it('turns the looping motion off under reduced motion', () => {
    const reduced = css.slice(css.indexOf('@media (prefers-reduced-motion: reduce)'));
    for (const name of ['motion-loop-ring', 'motion-loop-spin', 'motion-loop-edge']) {
      expect(reduced, name).toContain(`--orcon-${name}: 0ms;`);
    }
  });

  it('maps canvas sizes to spacing utilities', () => {
    const theme = emitTailwindTheme(model);
    expect(theme).toContain('--spacing-canvas-node: var(--orcon-canvas-node-size);');
    expect(theme).toContain('--spacing-canvas-label-width: var(--orcon-canvas-label-width);');
    expect(theme).toContain('--spacing-editor-pane-min: var(--orcon-editor-pane-min);');
    expect(theme).toContain('--radius-2xl: var(--orcon-radius-2xl);');
  });
});
