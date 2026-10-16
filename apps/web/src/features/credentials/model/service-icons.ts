// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/** The brand icons of the node types by type, as `listNodeTypes` answers them (inline SVG documents). */
export type ServiceIcons = Readonly<Record<string, { readonly svg: string; readonly darkSvg?: string | undefined }>>;

/** An SVG document as an address an `img` can show. The markup is sanitized by the server and only ever drawn through `img`, never inlined. */
function svgAddress(svg: string): string {
  return `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`;
}

/** The addresses of the icon of a connector, light and dark; empty when it has none. */
export function iconSources(icons: ServiceIcons | undefined, connector: string): { src: string | undefined; darkSrc: string | undefined } {
  const icon = icons?.[connector];
  return {
    src: icon === undefined ? undefined : svgAddress(icon.svg),
    darkSrc: icon?.darkSvg === undefined ? undefined : svgAddress(icon.darkSvg),
  };
}
