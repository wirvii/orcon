// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Avatar as AvatarPrimitive } from 'radix-ui';

import { cn } from '../../lib/cn';

export interface AvatarProps {
  /** Full name. It is the accessible name and the source of the initials. */
  name: string;
  /** The letters to show instead of the ones taken from the name, such as two letters of an email address. */
  initials?: string | undefined;
  /** Image URL. When it is missing or fails to load, the initials show. */
  src?: string | undefined;
  size?: 'sm' | 'md' | 'lg' | undefined;
  /** `accent` fills the avatar with the primary color, for the person who is signed in. */
  tone?: 'neutral' | 'accent' | undefined;
  /** Next to the name of the same person: the avatar is then hidden from assistive technology instead of repeating it. */
  decorative?: boolean | undefined;
  className?: string | undefined;
}

const sizeClass = { sm: 'size-icon-24 text-xs', md: 'size-connector-icon text-sm', lg: 'size-row text-base' } as const;

/** Up to two initials from a name: "Ada Lovelace" becomes "AL". */
export function initials(name: string): string {
  const words = name.trim().split(/\s+/).filter(Boolean);
  const first = words[0]?.[0] ?? '';
  const last = words.length > 1 ? (words[words.length - 1]?.[0] ?? '') : '';
  return (first + last).toUpperCase();
}

export function Avatar({ name, src, initials: letters, size = 'md', tone = 'neutral', decorative = false, className }: AvatarProps) {
  return (
    <AvatarPrimitive.Root
      role={decorative ? undefined : 'img'}
      aria-label={decorative ? undefined : name}
      aria-hidden={decorative || undefined}
      className={cn('relative inline-flex shrink-0 overflow-hidden rounded-full select-none', sizeClass[size], className)}
    >
      {src ? <AvatarPrimitive.Image src={src} alt="" className="size-full object-cover" /> : null}
      <AvatarPrimitive.Fallback
        aria-hidden
        className={cn(
          'flex size-full items-center justify-center font-medium',
          tone === 'accent' ? 'bg-action-primary text-action-primary-fg' : 'bg-surface-sunken text-fg',
        )}
      >
        {letters ?? initials(name)}
      </AvatarPrimitive.Fallback>
    </AvatarPrimitive.Root>
  );
}
