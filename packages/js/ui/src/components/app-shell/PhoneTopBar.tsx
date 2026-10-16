// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { ReactNode } from 'react';

import { BrandMark } from '../brand-mark/BrandMark';

export interface PhoneTopBarProps {
  /** The organization being used, in small type above the section. Always readable. */
  context: string;
  /** The section or page being shown. */
  title: string;
  /** Controls at the end, such as the account button. */
  actions?: ReactNode | undefined;
}

/**
 * The bar across the top of a phone: the symbol, where you are (the organization above, the section below) and the
 * account. There is no menu button: the destinations are in the tab bar at the bottom.
 */
export function PhoneTopBar({ context, title, actions }: PhoneTopBarProps) {
  return (
    <header className="bg-surface border-border flex min-h-target shrink-0 items-center gap-2 border-b px-3 py-2">
      <BrandMark variant="symbol" />
      <div className="flex min-w-0 flex-1 flex-col">
        <span data-truncate className="text-fg-subtle truncate text-2xs">
          {context}
        </span>
        <span data-truncate className="text-fg truncate text-md font-semibold">
          {title}
        </span>
      </div>
      {actions ? <div className="flex shrink-0 items-center gap-1">{actions}</div> : null}
    </header>
  );
}
