// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { LucideIcon } from 'lucide-react';
import type { AnchorHTMLAttributes, ReactNode, Ref } from 'react';

import { cn } from '../../lib/cn';
import { Tooltip } from '../tooltip/Tooltip';

export interface NavItemProps extends Omit<AnchorHTMLAttributes<HTMLAnchorElement>, 'children'> {
  ref?: Ref<HTMLAnchorElement>;
  icon: LucideIcon;
  /** The destination name. Always present: with `collapsed` it becomes hidden text plus a tooltip. */
  label: string;
  /** The page being shown. Marks the item with aria-current, a heavier weight and a leading bar. */
  current?: boolean;
  /** Icon only (a navigation rail). The label stays available to assistive technology and as a tooltip. */
  collapsed?: boolean;
  /** Count of things waiting in that area. Shown next to the label and read after it. */
  badge?: ReactNode;
  /** Show the label from the lg breakpoint up and only the icon below it. Used by the responsive shell. */
  responsiveLabel?: boolean;
}

/** One entry of the main navigation. */
export function NavItem({
  icon: Icon,
  label,
  current = false,
  collapsed = false,
  badge,
  responsiveLabel = false,
  className,
  ref,
  ...props
}: NavItemProps) {
  const labelClass = collapsed ? 'sr-only' : responsiveLabel ? 'sr-only lg:not-sr-only' : undefined;
  const hasBadge = badge !== undefined && badge !== null;
  const link = (
    <a
      ref={ref}
      aria-current={current ? 'page' : undefined}
      className={cn(
        'text-fg-muted flex min-h-target w-full items-center gap-3 py-2 rounded-md border-l-2 border-transparent px-control-x text-base transition-colors',
        'hover:bg-surface-sunken hover:text-fg',
        current && 'bg-surface-sunken text-fg border-action-primary font-semibold',
        (collapsed || responsiveLabel) && 'justify-center px-0',
        responsiveLabel && 'lg:justify-start lg:px-control-x',
        className,
      )}
      {...props}
    >
      <Icon aria-hidden className="size-icon-20 shrink-0" />
      <span className={cn('min-w-0 wrap-anywhere', labelClass)}>{label}</span>
      {hasBadge ? (
        <span
          className={cn(
            'bg-status-attention-bg text-status-attention min-w-icon-20 ml-auto inline-flex items-center justify-center rounded-full px-1 text-xs font-medium',
            (collapsed || responsiveLabel) && 'sr-only',
            responsiveLabel && 'lg:not-sr-only',
          )}
        >
          {badge}
        </span>
      ) : null}
    </a>
  );

  return collapsed ? (
    <Tooltip content={label} side="right">
      {link}
    </Tooltip>
  ) : (
    link
  );
}
