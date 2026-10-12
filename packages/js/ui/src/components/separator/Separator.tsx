// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Separator as SeparatorPrimitive } from 'radix-ui';
import type { ComponentProps } from 'react';

import { cn } from '../../lib/cn';

export type SeparatorProps = ComponentProps<typeof SeparatorPrimitive.Root>;

/** A thin divider. Decorative by default (no role); set decorative={false} when it separates meaningful groups. */
export function Separator({ className, orientation = 'horizontal', decorative = true, ...props }: SeparatorProps) {
  return (
    <SeparatorPrimitive.Root
      orientation={orientation}
      decorative={decorative}
      className={cn(
        'shrink-0 border-border',
        orientation === 'horizontal' ? 'w-full border-t' : 'self-stretch border-l',
        className,
      )}
      {...props}
    />
  );
}
