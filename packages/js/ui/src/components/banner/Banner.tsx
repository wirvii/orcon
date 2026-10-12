// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { cva } from 'class-variance-authority';
import { BellRing, CircleAlert, CircleCheck, Info, TriangleAlert, type LucideIcon } from 'lucide-react';
import type { HTMLAttributes, ReactNode } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { cn } from '../../lib/cn';

export type BannerTone = 'info' | 'success' | 'warning' | 'danger' | 'attention';

const messages = defineMessages({
  info: { id: 'common.banner.info', defaultMessage: 'Information', description: 'Screen reader prefix of an informational banner.' },
  success: { id: 'common.banner.success', defaultMessage: 'Success', description: 'Screen reader prefix of a success banner.' },
  warning: { id: 'common.banner.warning', defaultMessage: 'Warning', description: 'Screen reader prefix of a warning banner.' },
  danger: { id: 'common.banner.danger', defaultMessage: 'Error', description: 'Screen reader prefix of an error banner.' },
  attention: {
    id: 'common.banner.attention',
    defaultMessage: 'Needs attention',
    description: 'Screen reader prefix of a banner about something that needs a person to decide.',
  },
});

const icons: Record<BannerTone, LucideIcon> = {
  info: Info,
  success: CircleCheck,
  warning: TriangleAlert,
  danger: CircleAlert,
  attention: BellRing,
};

// Every text in a banner uses the family color, which is certified on the family background.
const bannerVariants = cva('flex items-start gap-3 rounded-md border p-3 text-base wrap-anywhere', {
  variants: {
    tone: {
      info: 'border-status-neutral bg-status-neutral-bg text-status-neutral',
      success: 'border-status-success bg-status-success-bg text-status-success',
      warning: 'border-status-warning bg-status-warning-bg text-status-warning',
      danger: 'border-status-danger bg-status-danger-bg text-status-danger',
      attention: 'border-status-attention bg-status-attention-bg text-status-attention',
    },
  },
  defaultVariants: { tone: 'info' },
});

export interface BannerProps extends Omit<HTMLAttributes<HTMLDivElement>, 'title'> {
  tone?: BannerTone;
  /** Short statement of what happened or what is needed. */
  title: ReactNode;
  /** Extra buttons or links. Decisions never live in toasts; they live in banners and dialogs. */
  action?: ReactNode;
}

/**
 * A persistent message inside the page. The tone is shown by an icon and a
 * hidden text prefix as well as by color. Errors are announced immediately
 * (role alert); the other tones politely (role status).
 */
export function Banner({ tone = 'info', title, action, className, children, ...props }: BannerProps) {
  const intl = useIntl();
  const Icon = icons[tone];
  return (
    <div role={tone === 'danger' ? 'alert' : 'status'} className={cn(bannerVariants({ tone }), className)} {...props}>
      <Icon aria-hidden className="size-icon-20 shrink-0" />
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <p className="font-semibold">
          <span className="sr-only">{intl.formatMessage(messages[tone])}: </span>
          {title}
        </p>
        {children ? <div className="text-base">{children}</div> : null}
        {action ? <div className="mt-1 flex flex-wrap gap-2">{action}</div> : null}
      </div>
    </div>
  );
}
