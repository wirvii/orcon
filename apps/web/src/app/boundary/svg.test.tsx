// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from 'vitest';

import { MAX_SVG_LENGTH, sanitizeSvg, svgDataUrl } from './svg';

describe('sanitizeSvg', () => {
  it('keeps a plain brand mark', () => {
    const clean = sanitizeSvg('<svg viewBox="0 0 24 24" role="img"><title>WhatsApp</title><path fill="none" d="M0 0h24v24H0z"/></svg>');
    expect(clean).toContain('viewBox="0 0 24 24"');
    expect(clean).toContain('<path fill="none" d="M0 0h24v24H0z"></path>');
    expect(clean).toContain('xmlns="http://www.w3.org/2000/svg"');
  });

  it('drops scripts, event handlers, foreign content and outside references', () => {
    const clean = sanitizeSvg(
      '<svg viewBox="0 0 24 24" onload="alert(1)"><script>alert(1)</script><foreignObject><div>x</div></foreignObject><image href="https://evil.test/a.png"/><path d="M0 0" fill="url(https://evil.test/x)" onclick="alert(2)"/><a href="javascript:alert(3)"><path d="M1 1"/></a></svg>',
    );
    expect(clean).toBeDefined();
    expect(clean).not.toMatch(/script|onload|onclick|foreignobject|image|evil|javascript|<a/i);
    expect(clean).toContain('<path d="M0 0"></path>');
  });

  it('keeps references inside the same image', () => {
    const clean = sanitizeSvg('<svg viewBox="0 0 1 1"><defs><linearGradient id="g"><stop offset="0" stop-color="currentColor"/></linearGradient></defs><path d="M0 0" fill="url(#g)"/></svg>');
    expect(clean).toContain('fill="url(#g)"');
  });

  it('refuses text that is not an SVG, is empty or is too long', () => {
    expect(sanitizeSvg('')).toBeUndefined();
    expect(sanitizeSvg('<html><body/></html>')).toBeUndefined();
    expect(sanitizeSvg('not xml at all')).toBeUndefined();
    expect(sanitizeSvg(`<svg>${'<g/>'.repeat(MAX_SVG_LENGTH)}</svg>`)).toBeUndefined();
  });

  it('gives an image source for an accepted text and nothing for a refused one', () => {
    expect(svgDataUrl('<svg viewBox="0 0 1 1"><path d="M0 0"/></svg>')).toMatch(/^data:image\/svg\+xml;charset=utf-8,/);
    expect(svgDataUrl('nope')).toBeUndefined();
  });
});
