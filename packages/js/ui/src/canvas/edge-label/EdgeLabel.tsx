// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Repeat } from 'lucide-react';
import { useIntl } from 'react-intl';

import { cn } from '../../lib/cn';
import { canvasMessages } from '../messages';
import type { EdgeState } from '../model';
import { edgeLooks, familyClasses } from '../state';

export interface EdgeLabelProps {
  /** Items that travelled the connection in the execution being shown. */
  items?: number | undefined;
  /** The connection state. An uncertain connection shows "unknown" instead of a count. */
  state?: EdgeState | undefined;
  /** The connection comes from a pinned node. */
  pinned?: boolean | undefined;
  /** The connection goes back to an earlier node: a Repeat chip leads the label. */
  loopBack?: boolean | undefined;
  className?: string | undefined;
}

/** The text of a connection label: an ICU item count, "unknown" when uncertain, " · pinned" for pinned data. */
export function useEdgeLabelText({ items = 0, state, pinned = false }: Pick<EdgeLabelProps, 'items' | 'state' | 'pinned'>): string {
  const intl = useIntl();
  const base =
    state === 'uncertain' ? intl.formatMessage(canvasMessages.edgeUnknown) : intl.formatMessage(canvasMessages.edgeItems, { count: items });
  return pinned ? intl.formatMessage(canvasMessages.edgePinned, { label: base }) : base;
}

/**
 * The pill at the middle of a connection after a run: how many items went through, in the color of
 * the connection state. The text, the dash pattern of the line and the node names carry the same
 * information, so color is never the only signal.
 */
export function EdgeLabel({ items, state, pinned, loopBack = false, className }: EdgeLabelProps) {
  const intl = useIntl();
  const text = useEdgeLabelText({ items, state, pinned });
  const family = state && edgeLooks[state].family ? familyClasses[edgeLooks[state].family] : undefined;
  return (
    <span
      data-edge-label={state ?? 'design'}
      className={cn(
        'bg-canvas-node-bg inline-flex min-h-5 items-center gap-1 rounded-full border px-2 text-xs font-medium whitespace-nowrap shadow-sm',
        family ? [family.border, family.text] : 'border-canvas-node-border text-fg-muted',
        className,
      )}
    >
      {loopBack ? (
        <>
          <Repeat aria-hidden className="size-3 shrink-0" />
          <span>{intl.formatMessage(canvasMessages.edgeRepeat)}</span>
          <span aria-hidden>·</span>
        </>
      ) : null}
      {text}
    </span>
  );
}
