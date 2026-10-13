// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Ban, Check, CircleAlert, CircleDashed, LoaderCircle, ShieldQuestion, X } from 'lucide-react';
import type { LucideIcon } from 'lucide-react';

import { statusMap, type StatusKey } from '../../components/status-badge/status-map';
import { cn } from '../../lib/cn';
import { familyClasses } from '../state';

/** The glyph inside the 22 px circle. It is a shape signal, so Succeeded needs no text on the canvas. */
const glyphs: Partial<Record<StatusKey, LucideIcon>> = {
  SUCCEEDED: Check,
  SUCCEEDED_WITH_ERRORS: CircleAlert,
  FAILED: X,
  ERROR_FINAL: X,
  UNCERTAIN: ShieldQuestion,
  RUNNING: LoaderCircle,
  CANCELED: Ban,
  NOT_RUN: CircleDashed,
};

export interface NodeStatusBadgeProps {
  status: StatusKey;
  /** Place the badge on the top-right corner of its parent (offset -8, -8). Off for a standalone badge. */
  corner?: boolean | undefined;
  /** Scale the badge for low zoom levels. */
  className?: string | undefined;
}

/**
 * The execution state of a node as a 22 px circle on the tile corner: a solid circle in the state
 * family, the state glyph in the family background, and a ring in the canvas color so it reads on any
 * background. It is hidden from assistive technology; the node's accessible name carries the state.
 */
export function NodeStatusBadge({ status, corner = true, className }: NodeStatusBadgeProps) {
  const entry = statusMap[status];
  const family = familyClasses[entry.family];
  const Glyph = glyphs[status] ?? entry.icon;
  return (
    <span
      aria-hidden
      data-node-badge={status}
      className={cn(
        'border-canvas z-10 grid size-canvas-badge shrink-0 place-items-center rounded-full border-2',
        family.solid,
        family.onSolid,
        corner && 'absolute -top-2 -right-2',
        className,
      )}
    >
      <Glyph strokeWidth={3} className={cn('size-canvas-badge-glyph', entry.spins && 'motion-safe:animate-loop-spin')} />
    </span>
  );
}
