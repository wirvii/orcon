// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { HTMLAttributes } from 'react';

import { cn } from '../../lib/cn';

/** A keyboard key. Use one <Kbd> per key; render combinations with <KbdCombination>. */
export function Kbd({ className, ...props }: HTMLAttributes<HTMLElement>) {
  return (
    <kbd
      className={cn(
        'bg-surface-sunken text-fg border-control inline-flex min-w-icon-20 items-center justify-center rounded-sm border px-1 font-mono text-mono',
        className,
      )}
      {...props}
    />
  );
}

/** Keys pressed together, for example Ctrl + K. */
export function KbdCombination({ keys, className }: { keys: readonly string[]; className?: string }) {
  return (
    <span className={cn('inline-flex items-center gap-1', className)}>
      {keys.map((key, index) => (
        <span key={key} className="inline-flex items-center gap-1">
          {index > 0 ? (
            <span aria-hidden className="text-fg-muted">
              +
            </span>
          ) : null}
          <Kbd>{key}</Kbd>
        </span>
      ))}
    </span>
  );
}
