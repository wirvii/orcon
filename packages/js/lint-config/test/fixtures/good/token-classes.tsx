// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { cn } from './cn';

export const View = ({ open }: { open: boolean }) => (
  <div
    className={cn(
      'bg-surface text-fg-muted p-4 gap-2 hover:bg-action-primary data-[state=open]:p-6 md:p-8 h-control w-full',
      open && 'ring-focus',
    )}
  />
);
