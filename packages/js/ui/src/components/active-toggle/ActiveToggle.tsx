// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Switch as SwitchPrimitive } from 'radix-ui';
import { useId } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { cn } from '../../lib/cn';
import { Spinner } from '../spinner/Spinner';
import { Tooltip } from '../tooltip/Tooltip';

const messages = defineMessages({
  active: {
    id: 'activeToggle.active',
    defaultMessage: 'Active',
    description: 'Word next to the switch of a flow that runs when its triggers fire.',
  },
  inactive: {
    id: 'activeToggle.inactive',
    defaultMessage: 'Inactive',
    description: 'Word next to the switch of a flow that does not run.',
  },
  updating: {
    id: 'activeToggle.updating',
    defaultMessage: 'Updating',
    description: 'Word next to the switch of a flow while the server is changing whether it is active.',
  },
});

export interface ActiveToggleProps {
  /** What the server says: the flow is active. The switch only ever shows this, never a guess. */
  checked: boolean;
  /** Asks to change it. The switch moves only when `checked` changes, after the server has answered. */
  onCheckedChange: (next: boolean) => void;
  /** The server is answering a change. The switch stays where it is and says so. */
  busy?: boolean | undefined;
  /** Why it cannot be changed (for example, the flow was never published). The switch is blocked and states the reason. */
  disabledReason?: string | undefined;
  /** Names the switch for assistive technology: the flow it belongs to. */
  label: string;
  className?: string | undefined;
}

/**
 * The Active switch of a flow, with its word next to it: Active in green, Inactive in grey. It never flips
 * ahead of the server: while a change is in flight it shows "Updating" and keeps the old position. A switch
 * that cannot be turned on says why, to a pointer, to the keyboard and to a screen reader.
 */
export function ActiveToggle({ checked, onCheckedChange, busy = false, disabledReason, label, className }: ActiveToggleProps) {
  const intl = useIntl();
  const reasonId = useId();
  const blocked = disabledReason !== undefined;
  const word = busy ? messages.updating : checked ? messages.active : messages.inactive;

  const control = (
    <SwitchPrimitive.Root
      checked={checked}
      onCheckedChange={(next) => {
        if (blocked || busy) return;
        onCheckedChange(next);
      }}
      aria-label={label}
      aria-busy={busy || undefined}
      aria-disabled={blocked || undefined}
      aria-describedby={blocked ? reasonId : undefined}
      className={cn(
        'group inline-flex min-h-target min-w-target items-center justify-center rounded-md',
        blocked && 'cursor-not-allowed',
      )}
    >
      <span
        className={cn(
          'border-control bg-surface-sunken flex h-icon-20 w-8 shrink-0 items-center rounded-full border px-1 transition-colors',
          'group-data-[state=checked]:border-action-primary group-data-[state=checked]:bg-action-primary',
          blocked && 'border-border',
        )}
      >
        <SwitchPrimitive.Thumb
          className={cn(
            'bg-fg-muted size-3 rounded-full transition-transform',
            'data-[state=checked]:bg-action-primary-fg data-[state=checked]:translate-x-3',
            blocked && 'bg-fg-disabled',
          )}
        />
      </span>
    </SwitchPrimitive.Root>
  );

  return (
    <span className={cn('inline-flex items-center gap-2 text-xs font-semibold', className)}>
      {blocked ? <Tooltip content={disabledReason}>{control}</Tooltip> : control}
      <span className={cn('inline-flex min-w-12 items-center gap-1', checked && !busy ? 'text-status-success' : 'text-fg-muted')}>
        {busy ? <Spinner decorative size={16} /> : null}
        {intl.formatMessage(word)}
      </span>
      {blocked ? (
        <span id={reasonId} className="sr-only">
          {disabledReason}
        </span>
      ) : null}
    </span>
  );
}
