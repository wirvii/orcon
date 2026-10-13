// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Link as RouterLink } from '@tanstack/react-router';
import { Button, Mascot } from '@orcon/ui';
import { defineMessages, useIntl } from 'react-intl';

const messages = defineMessages({
  title: {
    id: 'app.notFound.title',
    defaultMessage: 'Page not found',
    description: 'Main heading of the page shown when the address does not exist.',
  },
  text: {
    id: 'app.notFound.text',
    defaultMessage: 'The page you asked for does not exist or was moved. Check the address, or go back to the start page.',
    description: 'Explains the page not found and what to do next.',
  },
  home: {
    id: 'app.notFound.home',
    defaultMessage: 'Go to the start page',
    description: 'Button on the page not found that goes to the start page.',
  },
});

/** The page shown when no route matches (orcon-web RT-7). */
export function NotFound() {
  const intl = useIntl();
  return (
    <div className="mx-auto flex max-w-screen-md flex-col items-center gap-4 py-8 text-center">
      <Mascot pose="explorer" />
      <h1 className="text-2xl font-semibold wrap-anywhere">{intl.formatMessage(messages.title)}</h1>
      <p className="text-fg-muted max-w-screen-sm wrap-anywhere">{intl.formatMessage(messages.text)}</p>
      <Button asChild>
        <RouterLink to="/">{intl.formatMessage(messages.home)}</RouterLink>
      </Button>
    </div>
  );
}
