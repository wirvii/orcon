// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Command, useCommandState } from 'cmdk';
import type { ReactNode } from 'react';

import { cn } from './cn';

/**
 * The results part of a cmdk list. When nothing matches, the (then empty)
 * listbox is hidden and the "no matches" notice stands on its own, because a
 * listbox without options is not valid ARIA.
 */
export function CommandResults({
  children,
  emptyMessage,
  emptyExtra,
  className,
  emptyClassName,
}: {
  children: ReactNode;
  emptyMessage: string;
  /** What follows the notice when nothing matches (the way out, for example). */
  emptyExtra?: ReactNode;
  className?: string;
  emptyClassName?: string;
}) {
  const count = useCommandState((state) => state.filtered.count);
  return (
    <>
      <Command.List hidden={count === 0} className={className}>
        {children}
      </Command.List>
      {count === 0 ? (
        <>
          <p role="status" className={cn(emptyClassName)}>
            {emptyMessage}
          </p>
          {emptyExtra}
        </>
      ) : null}
    </>
  );
}
