// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import happy from '../../assets/mascot/mascot-happy.png';
import crossedArms from '../../assets/mascot/mascot-crossed-arms.png';
import thinking from '../../assets/mascot/mascot-thinking.png';
import { cn } from '../../lib/cn';

/**
 * The approved poses and the cut-out each one shows (brand/manifest.json):
 * `welcome` is the crossed-arms cut-out, `happy` the laughing expression and
 * `thinking` the thinking expression. The size is the intrinsic size of the file.
 */
const poses = {
  welcome: { src: crossedArms, width: 412, height: 374 },
  happy: { src: happy, width: 193, height: 204 },
  thinking: { src: thinking, width: 196, height: 204 },
} as const;

export type MascotPose = keyof typeof poses;

export interface MascotProps {
  pose?: MascotPose | undefined;
  className?: string | undefined;
}

/**
 * The approved mascot on a soft rounded panel (the `mascot.panel` token, lighter
 * in the dark theme so the dark question mark of the thinking pose stays legible).
 * The cut-outs are never redrawn, simplified or recolored; a new pose comes only
 * from a new approved cut-out. The mascot is decorative: it is hidden from
 * assistive technology, never carries text and never speaks. The text next to it
 * carries the meaning. The image is at most 160 px tall (`.orcon-mascot-image`
 * in styles/index.css) and loads lazily.
 *
 * Allowed only on the sign-in and welcome screens, first-run empty states, a
 * successful n8n import and full-page error screens (not found, generic error
 * page). Never on inline errors, security warnings, the canvas or dense views
 * (tables, execution detail, the inbox); the lint configuration rejects the import
 * there.
 */
export function Mascot({ pose = 'welcome', className }: MascotProps) {
  const { src, width, height } = poses[pose];
  return (
    <div aria-hidden data-mascot-pose={pose} className={cn('bg-mascot-panel inline-flex shrink-0 items-end justify-center rounded-2xl p-3', className)}>
      <img src={src} width={width} height={height} alt="" aria-hidden loading="lazy" decoding="async" className="orcon-mascot-image" />
    </div>
  );
}
