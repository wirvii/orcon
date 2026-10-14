// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { readFileSync } from 'node:fs';
import { join } from 'node:path';

import { tokens } from '@orcon/tokens';
import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { BrandHero } from '../../src/components/brand-hero/BrandHero';
import { BrandMark } from '../../src/components/brand-mark/BrandMark';
import { Mascot, type MascotPose } from '../../src/components/mascot/Mascot';

import { renderWithProviders } from './helpers';

const source = (path: string) => readFileSync(join(import.meta.dirname, '../../src', path), 'utf8');
const svg = (name: string) => source(`assets/brand/${name}`);
const stylesheet = source('styles/index.css');

/** The `src` attributes of the images inside an element, in document order. */
const sources = (element: Element) => [...element.querySelectorAll('img')].map((img) => img.getAttribute('src') ?? '');

describe('BrandMark', () => {
  it('renders the approved lockup named Orcon', () => {
    renderWithProviders(<BrandMark />);
    const mark = screen.getByRole('img', { name: 'Orcon' });
    expect(sources(mark)).toEqual([expect.stringContaining('orcon-lockup-on-dark.svg'), expect.stringContaining('orcon-lockup-on-light.svg')]);
    for (const img of mark.querySelectorAll('img')) {
      expect(img).toHaveAttribute('alt', '');
      expect(img).toHaveAttribute('aria-hidden', 'true');
    }
  });

  it('renders only the symbol in the symbol variant', () => {
    renderWithProviders(<BrandMark variant="symbol" />);
    const mark = screen.getByRole('img', { name: 'Orcon' });
    expect(sources(mark)).toEqual([expect.stringContaining('orcon-symbol.svg'), expect.stringContaining('orcon-app-icon.svg')]);
    expect(sources(mark).join()).not.toContain('lockup');
  });

  it('has a light and a dark variant chosen by the theme', () => {
    renderWithProviders(<BrandMark />);
    const mark = screen.getByRole('img', { name: 'Orcon' });
    expect(mark.querySelector('img.orcon-brand-on-dark')).not.toBeNull();
    expect(mark.querySelector('img.orcon-brand-on-light')).not.toBeNull();
    // CSS picks the variant with the selectors the tokens use: the saved theme, else the system setting.
    expect(stylesheet).toContain("[data-theme='dark'] .orcon-brand-on-dark");
    expect(stylesheet).toContain("[data-theme='light'] .orcon-brand-on-light");
    expect(stylesheet).toMatch(/prefers-color-scheme: dark\)[^{]*\{\s*:root:not\(\[data-theme='light'\]\) \.orcon-brand-on-dark/);
  });

  it('is not a placeholder', () => {
    const { container } = renderWithProviders(<BrandMark />);
    expect(container.querySelector('[data-placeholder]')).toBeNull();
    expect(container.querySelectorAll('img')).toHaveLength(2);
  });
});

describe('BrandHero', () => {
  it('shows the descriptor lockup named Orcon and the tagline as text', () => {
    const { container } = renderWithProviders(<BrandHero />);
    const lockup = screen.getByRole('img', { name: 'Orcon' });
    expect(sources(lockup)).toEqual([
      expect.stringContaining('orcon-lockup-descriptor-on-dark.svg'),
      expect.stringContaining('orcon-lockup-descriptor-on-light.svg'),
    ]);
    // The tagline is real text, not part of an image.
    expect(screen.getByText('Build. Orchestrate. Execute. Your way.').tagName).toBe('P');
    expect(container.querySelectorAll('img')).toHaveLength(2);
  });
});

const poseFiles: Record<MascotPose, string> = {
  welcome: 'mascot-crossed-arms.png',
  happy: 'mascot-happy.png',
  thinking: 'mascot-thinking.png',
};

describe('Mascot', () => {
  it('renders the approved cut-out for each pose', () => {
    for (const [pose, file] of Object.entries(poseFiles) as [MascotPose, string][]) {
      const { container, unmount } = renderWithProviders(<Mascot pose={pose} />);
      expect(container.querySelector(`[data-mascot-pose="${pose}"]`)).not.toBeNull();
      expect(container.querySelector('img')?.getAttribute('src')).toContain(file);
      unmount();
    }
  });

  it('is decorative, lazy and at most 160 px tall', () => {
    const { container } = renderWithProviders(<Mascot pose="thinking" />);
    const panel = container.querySelector('[data-mascot-pose]');
    const image = container.querySelector('img');
    expect(panel).toHaveAttribute('aria-hidden', 'true');
    expect(image).toHaveAttribute('alt', '');
    expect(image).toHaveAttribute('aria-hidden', 'true');
    expect(image).toHaveAttribute('loading', 'lazy');
    expect(image).toHaveAttribute('decoding', 'async');
    // The intrinsic size is declared so the page does not move when the image arrives.
    expect(image).toHaveAttribute('width');
    expect(image).toHaveAttribute('height');
    expect(panel?.textContent).toBe('');
    const height = /\.orcon-mascot-image\s*\{[^}]*height:\s*(\d+)px/.exec(stylesheet);
    expect(Number(height?.[1])).toBeLessThanOrEqual(160);
  });

  it('is not a placeholder', () => {
    const { container } = renderWithProviders(<Mascot />);
    expect(container.querySelector('[data-placeholder]')).toBeNull();
    expect(container.querySelector('img')?.getAttribute('src')).toMatch(/\.png$/);
  });
});

/** WCAG 2.x contrast ratio of two #RRGGBB colors. */
function ratio(a: string, b: string): number {
  const luminance = (hex: string) => {
    const n = Number.parseInt(hex.slice(1), 16);
    const channel = (value: number) => {
      const c = value / 255;
      return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
    };
    return 0.2126 * channel((n >> 16) & 255) + 0.7152 * channel((n >> 8) & 255) + 0.0722 * channel(n & 255);
  };
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x) as [number, number];
  return (hi + 0.05) / (lo + 0.05);
}

/** The gradient stop colors of an SVG, in document order. */
const stops = (file: string) => [...svg(file).matchAll(/stop-color="(#[0-9A-Fa-f]{6})"/g)].map((m) => m[1] as string);
/** The solid fills of an SVG (the wordmark and the tile), without the gradient references. */
const solidFills = (file: string) => [...new Set([...svg(file).matchAll(/fill="(#[0-9A-Fa-f]{6})"/g)].map((m) => m[1] as string))];

const surface = (theme: 'light' | 'dark') => (tokens.themes[theme] as Record<string, string>)['surface.default'] as string;

describe('Brand contrast', () => {
  it('the wordmark meets 4.5:1 on the top bar surface in both themes', () => {
    // The top bar sits on surface.default. Dark theme: the lockup with light type. Light theme: navy type.
    const dark = solidFills('orcon-lockup-on-dark.svg');
    const light = solidFills('orcon-lockup-on-light.svg');
    expect(dark.length).toBeGreaterThan(0);
    expect(light.length).toBeGreaterThan(0);
    for (const color of dark) expect(ratio(color, surface('dark')), `wordmark ${color} on dark`).toBeGreaterThanOrEqual(4.5);
    for (const color of light) expect(ratio(color, surface('light')), `wordmark ${color} on light`).toBeGreaterThanOrEqual(4.5);
  });

  it('the symbol meets 3:1 against what is behind it in both themes', () => {
    // Dark theme: every gradient stop of the symbol against the surface.
    const darkStops = stops('orcon-symbol.svg');
    // The board gradient: the light lime highlight, the green and the cyan.
    expect(new Set(darkStops).size).toBe(3);
    for (const color of darkStops) expect(ratio(color, surface('dark')), `stop ${color} on dark`).toBeGreaterThanOrEqual(3);
    // Light theme (decision Q2 = A): the symbol sits inside its dark rounded square. The square against the
    // surface, then every stop against the square.
    const [tile] = solidFills('orcon-app-icon.svg');
    expect(tile).toBeDefined();
    expect(ratio(tile as string, surface('light')), 'tile on light').toBeGreaterThanOrEqual(3);
    for (const color of stops('orcon-app-icon.svg')) expect(ratio(color, tile as string), `stop ${color} on tile`).toBeGreaterThanOrEqual(3);
  });
});
