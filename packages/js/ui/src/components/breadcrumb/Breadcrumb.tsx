// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { ChevronRight } from 'lucide-react';
import { Fragment } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { Link } from '../link/Link';

const messages = defineMessages({
  label: {
    id: 'common.breadcrumb.label',
    defaultMessage: 'Breadcrumb',
    description: 'Accessible name of the navigation landmark that shows where the current page sits in the hierarchy.',
  },
});

export interface BreadcrumbItem {
  label: string;
  /** Omit on the last item, the current page. */
  href?: string;
}

export interface BreadcrumbProps {
  items: readonly BreadcrumbItem[];
  className?: string;
}

/** The path to the current page. The last item is the page itself and is not a link. */
export function Breadcrumb({ items, className }: BreadcrumbProps) {
  const intl = useIntl();
  return (
    <nav aria-label={intl.formatMessage(messages.label)} className={className}>
      <ol className="flex flex-wrap items-center gap-1 text-base">
        {items.map((item, index) => {
          const last = index === items.length - 1;
          return (
            <Fragment key={`${item.label}-${index}`}>
              <li className="flex items-center">
                {last || !item.href ? (
                  <span aria-current={last ? 'page' : undefined} className={last ? 'text-fg font-medium' : 'text-fg-muted'}>
                    {item.label}
                  </span>
                ) : (
                  <Link href={item.href}>{item.label}</Link>
                )}
              </li>
              {last ? null : (
                <li aria-hidden className="text-fg-muted flex items-center">
                  <ChevronRight className="size-icon-16" />
                </li>
              )}
            </Fragment>
          );
        })}
      </ol>
    </nav>
  );
}
