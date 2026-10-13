// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { cn } from '../../lib/cn';

/** Allowed poses: welcome (onboarding), thinking (empty states), thumbs-up (import success), explorer (404). */
export type MascotPose = 'welcome' | 'thinking' | 'thumbs-up' | 'explorer';

export interface MascotProps {
  pose?: MascotPose | undefined;
  className?: string | undefined;
}

/**
 * Neutral placeholder for the mascot: an empty outlined frame with no drawing,
 * until the production artwork is delivered and approved. It is decorative
 * (hidden from assistive technology); the text next to it carries the meaning.
 *
 * The mascot is only allowed in welcome and onboarding, empty states, a
 * successful import, 404 pages and docs. Never use it on errors, security
 * warnings, the canvas or dense views (tables, execution detail, inbox); the
 * lint configuration rejects the import there.
 */
export function Mascot({ pose = 'welcome', className }: MascotProps) {
  return (
    <div
      aria-hidden
      data-mascot-pose={pose}
      data-placeholder
      className={cn('border-control size-16 shrink-0 rounded-xl border border-dashed', className)}
    />
  );
}
