// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Menu } from 'lucide-react';
import type { ReactNode } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { cn } from '../../lib/cn';
import { IconButton } from '../icon-button/IconButton';

const messages = defineMessages({
  openNavigation: {
    id: 'nav.openMenu',
    defaultMessage: 'Open navigation menu',
    description: 'Accessible name of the button, shown on small screens, that opens the navigation menu.',
  },
});

export interface TopBarProps {
  /** Shown at the start, usually the BrandMark. */
  brand?: ReactNode | undefined;
  /** Shows the menu button, used below the md breakpoint where the sidebar is hidden. */
  onOpenNavigation?: (() => void) | undefined;
  /** Controls at the end: notification bell, theme and density, account. */
  actions?: ReactNode | undefined;
  className?: string | undefined;
}

/** The bar across the top of the app. It stays in view while the page scrolls. */
export function TopBar({ brand, onOpenNavigation, actions, className }: TopBarProps) {
  const intl = useIntl();
  return (
    <header className={cn('bg-surface border-border z-sticky sticky top-0 flex h-row min-h-target items-center gap-2 border-b px-3', className)}>
      {onOpenNavigation ? (
        <IconButton
          variant="ghost"
          icon={Menu}
          label={intl.formatMessage(messages.openNavigation)}
          onClick={onOpenNavigation}
          className="md:hidden"
        />
      ) : null}
      <div className="flex min-w-0 flex-1 items-center">{brand}</div>
      {actions ? <div className="flex shrink-0 items-center gap-1">{actions}</div> : null}
    </header>
  );
}
