// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { cva, type VariantProps } from 'class-variance-authority';
import type { HTMLAttributes, ReactNode } from 'react';

import { cn } from '../../lib/cn';

export const badgeVariants = cva(
  'inline-flex max-w-full items-center gap-1 rounded-full px-2 text-xs font-medium wrap-anywhere',
  {
    variants: {
      /** Color families are for meaning: use them only for status. `neutral` is the default. */
      tone: {
        neutral: 'bg-status-neutral-bg text-status-neutral',
        active: 'bg-status-active-bg text-status-active',
        success: 'bg-status-success-bg text-status-success',
        warning: 'bg-status-warning-bg text-status-warning',
        danger: 'bg-status-danger-bg text-status-danger',
        attention: 'bg-status-attention-bg text-status-attention',
      },
    },
    defaultVariants: { tone: 'neutral' },
  },
);

export interface BadgeProps extends HTMLAttributes<HTMLSpanElement>, VariantProps<typeof badgeVariants> {
  /** Optional leading icon (decorative). Status meaning must also be in the text. */
  icon?: ReactNode | undefined;
}

/** A small label. It is never the only carrier of meaning: the text says it, the color reinforces it. */
export function Badge({ className, tone, icon, children, ...props }: BadgeProps) {
  return (
    <span className={cn(badgeVariants({ tone }), 'min-h-icon-20', className)} {...props}>
      {icon}
      {children}
    </span>
  );
}
