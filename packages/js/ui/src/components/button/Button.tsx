// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { cva, type VariantProps } from 'class-variance-authority';
import { Slot } from 'radix-ui';
import type { ButtonHTMLAttributes, Ref } from 'react';

import { cn } from '../../lib/cn';
import { Spinner } from '../spinner/Spinner';

export const buttonVariants = cva(
  [
    'inline-flex min-w-target max-w-full items-center justify-center gap-2 rounded-md',
    'text-center text-base font-medium wrap-anywhere select-none transition-colors',
    'disabled:pointer-events-none disabled:border disabled:border-border disabled:bg-surface disabled:text-fg-disabled',
    'aria-disabled:pointer-events-none',
  ],
  {
    variants: {
      variant: {
        primary: 'bg-action-primary text-action-primary-fg hover:bg-action-primary-hover active:bg-action-primary-hover',
        secondary: 'border border-control bg-surface text-fg hover:bg-surface-sunken active:bg-surface-sunken',
        ghost: 'text-fg hover:bg-surface-sunken active:bg-surface-sunken',
        danger: 'bg-action-danger text-action-danger-fg hover:bg-action-danger-hover active:bg-action-danger-hover',
      },
      size: {
        // Height and padding come from the density tokens.
        default: 'min-h-control-target px-control-x py-1',
        icon: 'h-control min-h-target w-control',
      },
    },
    defaultVariants: { variant: 'primary', size: 'default' },
  },
);

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement>, VariantProps<typeof buttonVariants> {
  ref?: Ref<HTMLButtonElement> | undefined;
  /** Render the child element (for example a router link) with the button styling. */
  asChild?: boolean | undefined;
  /** Busy: the button stays focusable and announces the busy state, but ignores clicks. */
  loading?: boolean | undefined;
}

export function Button({ className, variant, size, asChild = false, loading = false, children, onClick, ref, ...props }: ButtonProps) {
  const Component = asChild ? Slot.Root : 'button';
  return (
    <Component
      ref={ref}
      type={asChild ? undefined : 'button'}
      aria-busy={loading || undefined}
      aria-disabled={loading || undefined}
      onClick={loading ? (event) => event.preventDefault() : onClick}
      className={cn(buttonVariants({ variant, size }), className)}
      {...props}
    >
      {asChild ? (
        children
      ) : (
        <>
          {loading ? <Spinner decorative className="text-current" /> : null}
          {children}
        </>
      )}
    </Component>
  );
}
