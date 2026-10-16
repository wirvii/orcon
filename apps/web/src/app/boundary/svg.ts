// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * The boundary for an icon that arrives as SVG text (`NodeType.icon`, SPEC-019 R-UX-ICO). The text is catalog
 * content, so it is never put in the page as markup: it is checked against an allow list, rewritten from the
 * checked nodes only, and handed to an <img> as a data address. An image cannot run script or load other
 * files, so the allow list is a second wall, not the only one.
 */

/** The longest SVG text accepted. A brand mark is a few hundred bytes to a few kilobytes. */
export const MAX_SVG_LENGTH = 20_000;

const ELEMENTS = new Set(['svg', 'g', 'path', 'circle', 'ellipse', 'rect', 'line', 'polyline', 'polygon', 'defs', 'lineargradient', 'radialgradient', 'stop', 'clippath', 'title']);

const ATTRIBUTES = new Set([
  'viewbox',
  'width',
  'height',
  'd',
  'fill',
  'fill-rule',
  'clip-rule',
  'fill-opacity',
  'opacity',
  'stroke',
  'stroke-width',
  'stroke-linecap',
  'stroke-linejoin',
  'stroke-opacity',
  'transform',
  'cx',
  'cy',
  'r',
  'rx',
  'ry',
  'x',
  'y',
  'x1',
  'y1',
  'x2',
  'y2',
  'points',
  'id',
  'offset',
  'stop-color',
  'stop-opacity',
  'gradientunits',
  'gradienttransform',
  'clip-path',
  'role',
]);

/** A value that points outside the image or carries script. Only same-document references (`url(#id)`) are kept. */
const unsafeValue = (value: string): boolean => /javascript:|data:|https?:|\/\/|expression\(|<|>/i.test(value) || /url\((?!\s*#)/i.test(value);

/**
 * The SVG text rebuilt from the elements and attributes the allow list names, or `undefined` when the text is not
 * a usable SVG (too long, not SVG, or nothing is left of it).
 */
export function sanitizeSvg(text: string): string | undefined {
  if (text.length === 0 || text.length > MAX_SVG_LENGTH) return undefined;
  const document = new DOMParser().parseFromString(text, 'image/svg+xml');
  const root = document.documentElement;
  if (document.querySelector('parsererror') !== null || root.localName.toLowerCase() !== 'svg') return undefined;

  const rebuild = (element: Element): string => {
    const name = element.localName.toLowerCase();
    if (!ELEMENTS.has(name)) return '';
    const attributes = Array.from(element.attributes)
      .filter((attribute) => ATTRIBUTES.has(attribute.localName.toLowerCase()) && !unsafeValue(attribute.value))
      .map((attribute) => ` ${attribute.localName}="${attribute.value.replace(/"/g, '&quot;')}"`)
      .join('');
    const inner = Array.from(element.childNodes)
      .map((child) => {
        if (child.nodeType === 1) return rebuild(child as Element);
        // Only the text of <title> is kept: it is the one element whose text is not a drawing.
        if (child.nodeType === 3 && name === 'title') return (child.textContent ?? '').replace(/[<>&]/g, '');
        return '';
      })
      .join('');
    return `<${name}${attributes}>${inner}</${name}>`;
  };

  const clean = rebuild(root);
  if (clean === '') return undefined;
  // The namespace is required for an image to render; it is added here, never read from the input.
  return clean.replace('<svg', '<svg xmlns="http://www.w3.org/2000/svg"');
}

/** An <img> source for sanitized SVG text, or `undefined` when the text is not accepted. */
export function svgDataUrl(text: string): string | undefined {
  const clean = sanitizeSvg(text);
  return clean === undefined ? undefined : `data:image/svg+xml;charset=utf-8,${encodeURIComponent(clean)}`;
}
