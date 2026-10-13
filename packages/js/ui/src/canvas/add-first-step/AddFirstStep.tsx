// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Plus } from 'lucide-react';
import { useIntl } from 'react-intl';

import { cn } from '../../lib/cn';
import { canvasMessages } from '../messages';

export interface AddFirstStepProps {
  /** Opens the node palette filtered to triggers. */
  onActivate?: (() => void) | undefined;
  className?: string | undefined;
}

/**
 * What an empty canvas shows: one dashed 100 px tile with a plus and the words "Add first step"
 * below it. It opens the node palette filtered to triggers.
 */
export function AddFirstStep({ onActivate, className }: AddFirstStepProps) {
  const intl = useIntl();
  return (
    <div className={cn('flex w-canvas-label-width flex-col items-center gap-2', className)}>
      <button
        type="button"
        onClick={onActivate}
        aria-label={intl.formatMessage(canvasMessages.addFirstStep)}
        className={cn(
          'nodrag nopan bg-canvas-node-bg border-canvas-edge text-fg-muted grid size-canvas-node place-items-center rounded-2xl border-2 border-dashed',
          'hover:border-action-primary hover:text-action-primary',
        )}
      >
        <Plus aria-hidden className="size-canvas-icon" />
      </button>
      <span aria-hidden className="text-fg text-sm font-semibold">
        {intl.formatMessage(canvasMessages.addFirstStep)}
      </span>
    </div>
  );
}
