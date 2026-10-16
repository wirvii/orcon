// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { svgDataUrl } from '../../../app/boundary/svg';
import { brandIconFor, brandOf, type NodeIconSource } from '../model/brand-marks';

/**
 * The icon of a node type. The inline SVG the node-type list serves (`NodeType.icon`) comes first, rebuilt by the
 * boundary sanitizer; without it, the vendored mark of the brand; without that, nothing (the canvas then shows
 * the generic plug, never an empty box).
 */
export function nodeIconFor(type: string, inlineSvg?: string): NodeIconSource | undefined {
  const served = inlineSvg === undefined ? undefined : svgDataUrl(inlineSvg);
  if (served !== undefined) return { src: served, name: brandIconFor(type)?.name ?? brandOf(type) ?? type };
  return brandIconFor(type);
}
