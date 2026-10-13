// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { ConnectorIcon } from '../components/connector-icon/ConnectorIcon';
import { cn } from '../lib/cn';

import { iconFor } from './icons';
import type { GraphNode } from './model';

export interface NodeGlyphProps {
  node: Pick<GraphNode, 'kind' | 'logicType' | 'icon' | 'connector'>;
  /** Size class of the Lucide icon (not used for connector logos, which fill their box). */
  iconClassName?: string | undefined;
  /** A quiet look for disabled and not-run nodes: the icon uses the subtle foreground. */
  muted?: boolean | undefined;
  className?: string | undefined;
}

/** The icon of a node: a connector logo (an <img>) or a built-in Lucide icon. Decorative: the name is next to it. */
export function NodeGlyph({ node, iconClassName = 'size-canvas-icon', muted = false, className }: NodeGlyphProps) {
  if (node.connector) return <ConnectorIcon variant="tile" src={node.connector.src} name={node.connector.name} className={className} />;
  const Icon = iconFor(node);
  const tone = muted ? 'text-fg-subtle' : node.kind === 'trigger' ? 'text-fg-accent' : node.kind === 'logic' ? 'text-fg-link' : 'text-fg';
  return <Icon aria-hidden className={cn(iconClassName, tone, className)} />;
}
