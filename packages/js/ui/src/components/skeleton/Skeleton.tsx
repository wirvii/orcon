// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { cva, type VariantProps } from 'class-variance-authority';
import type { HTMLAttributes } from 'react';

import { cn } from '../../lib/cn';

const skeletonVariants = cva('bg-surface-sunken motion-safe:animate-pulse-soft', {
  variants: {
    shape: {
      block: 'rounded-md',
      text: 'h-4 rounded-sm',
      circle: 'rounded-full',
    },
  },
  defaultVariants: { shape: 'block' },
});

export interface SkeletonProps extends HTMLAttributes<HTMLDivElement>, VariantProps<typeof skeletonVariants> {}

/**
 * A placeholder shown while content loads. It is hidden from assistive
 * technology: mark the loading region with aria-busy and announce it with a
 * status message instead. Give it a size with width and height utilities.
 */
export function Skeleton({ className, shape, ...props }: SkeletonProps) {
  return <div aria-hidden className={cn(skeletonVariants({ shape }), className)} {...props} />;
}
