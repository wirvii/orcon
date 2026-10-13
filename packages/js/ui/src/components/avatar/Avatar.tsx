// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Avatar as AvatarPrimitive } from 'radix-ui';

import { cn } from '../../lib/cn';

export interface AvatarProps {
  /** Full name. It is the accessible name and the source of the initials. */
  name: string;
  /** Image URL. When it is missing or fails to load, the initials show. */
  src?: string;
  size?: 'sm' | 'md' | 'lg';
  className?: string;
}

const sizeClass = { sm: 'size-icon-24 text-xs', md: 'size-connector-icon text-sm', lg: 'size-row text-base' } as const;

/** Up to two initials from a name: "Ada Lovelace" becomes "AL". */
export function initials(name: string): string {
  const words = name.trim().split(/\s+/).filter(Boolean);
  const first = words[0]?.[0] ?? '';
  const last = words.length > 1 ? (words[words.length - 1]?.[0] ?? '') : '';
  return (first + last).toUpperCase();
}

export function Avatar({ name, src, size = 'md', className }: AvatarProps) {
  return (
    <AvatarPrimitive.Root
      role="img"
      aria-label={name}
      className={cn('relative inline-flex shrink-0 overflow-hidden rounded-full select-none', sizeClass[size], className)}
    >
      {src ? <AvatarPrimitive.Image src={src} alt="" className="size-full object-cover" /> : null}
      <AvatarPrimitive.Fallback
        aria-hidden
        className="bg-surface-sunken text-fg flex size-full items-center justify-center font-medium"
      >
        {initials(name)}
      </AvatarPrimitive.Fallback>
    </AvatarPrimitive.Root>
  );
}
