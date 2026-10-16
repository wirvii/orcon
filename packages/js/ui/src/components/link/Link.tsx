// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { ExternalLink } from 'lucide-react';
import { Slot } from 'radix-ui';
import type { AnchorHTMLAttributes, Ref } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { cn } from '../../lib/cn';

const messages = defineMessages({
  opensInNewTab: {
    id: 'common.link.opensInNewTab',
    defaultMessage: '(opens in a new tab)',
    description: 'Screen reader text appended to a link that opens in a new browser tab.',
  },
});

export interface LinkProps extends AnchorHTMLAttributes<HTMLAnchorElement> {
  ref?: Ref<HTMLAnchorElement> | undefined;
  /**
   * `plain` (the default) is never underlined: navigation, rows, menus and actions. `prose` is underlined and
   * belongs only inside running text. A link inside `inText` text is `prose` unless said otherwise.
   */
  variant?: 'plain' | 'prose' | undefined;
  /** Opens in a new tab, shows an external icon and tells assistive technology. */
  external?: boolean | undefined;
  /**
   * The link sits inside running text. Its target size is then exempt, but only
   * when an ancestor carries the `data-prose` attribute.
   */
  inText?: boolean | undefined;
  /** Render the child element (a router link) with the link styling. */
  asChild?: boolean | undefined;
}

export function Link({ className, variant, external = false, inText = false, asChild = false, children, ref, ...props }: LinkProps) {
  const intl = useIntl();
  const Component = asChild ? Slot.Root : 'a';
  const prose = (variant ?? (inText ? 'prose' : 'plain')) === 'prose';
  const classes = cn(
    'text-fg-link rounded-sm transition-colors hover:text-fg active:text-fg',
    prose ? 'underline underline-offset-2' : 'no-underline',
    inText ? 'inline' : 'inline-flex min-h-target min-w-target items-center gap-1',
    className,
  );
  const externalProps = external ? { target: '_blank', rel: 'noopener noreferrer' } : {};

  if (asChild) {
    return (
      <Component ref={ref} data-variant={prose ? 'prose' : 'plain'} className={classes} {...externalProps} {...props}>
        {children}
      </Component>
    );
  }
  return (
    <Component ref={ref} data-variant={prose ? 'prose' : 'plain'} className={classes} {...externalProps} {...props}>
      {children}
      {external ? (
        <>
          <ExternalLink aria-hidden className="size-icon-16 shrink-0" />
          <span className="sr-only">{intl.formatMessage(messages.opensInNewTab)}</span>
        </>
      ) : null}
    </Component>
  );
}
