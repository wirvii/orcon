// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { cn } from '../../lib/cn';

/** The product name. It is a proper noun and is not translated. */
const PRODUCT_NAME = 'Orcon';

export interface BrandMarkProps {
  /** `lockup` shows the symbol and the name; `symbol` only the symbol. */
  variant?: 'lockup' | 'symbol' | undefined;
  className?: string | undefined;
}

/**
 * Neutral placeholder for the Orcon brand mark: a symbol-sized empty rounded
 * frame and the name in Inter 700. The approved symbol and wordmark replace the
 * frame later without changing this contract. Never redraw the symbol here.
 */
export function BrandMark({ variant = 'lockup', className }: BrandMarkProps) {
  return (
    <span data-placeholder className={cn('text-fg inline-flex items-center gap-2', className)} role="img" aria-label={PRODUCT_NAME}>
      <span aria-hidden className="border-control size-connector-icon shrink-0 rounded-md border" />
      {variant === 'lockup' ? (
        <span aria-hidden className="text-lg font-bold">
          {PRODUCT_NAME}
        </span>
      ) : null}
    </span>
  );
}
