// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { defineMessages, useIntl } from 'react-intl';

import { BrandMark } from '../components/brand-mark/BrandMark';
import { Button } from '../components/button/Button';
import { Link } from '../components/link/Link';
import { Mascot } from '../components/mascot/Mascot';

const messages = defineMessages({
  title: { id: 'screen.notFound.title', defaultMessage: 'Page not found', description: 'Main heading of the page shown when the address does not exist.' },
  text: {
    id: 'screen.notFound.text',
    defaultMessage: 'The page you asked for does not exist or was moved. Check the address, or go back to your flows.',
    description: 'Explains the page not found and what to do next.',
  },
  home: { id: 'screen.notFound.home', defaultMessage: 'Go to flows', description: 'Button on the page not found that goes to the list of flows.' },
  back: { id: 'screen.notFound.back', defaultMessage: 'Go back', description: 'Link on the page not found that goes to the previous page.' },
});

/** S8 (part 2): the 404 page. The thinking mascot is allowed on this full-page error screen. */
export function NotFoundScreen() {
  const intl = useIntl();
  return (
    <div className="bg-app text-fg flex min-h-screen flex-col">
      <header className="bg-surface border-border flex h-row min-h-target items-center border-b px-4">
        <BrandMark />
      </header>
      <main className="mx-auto flex w-full max-w-screen-md flex-1 flex-col items-center justify-center gap-4 p-4 text-center">
        <Mascot pose="thinking" />
        <h1 className="text-2xl font-semibold wrap-anywhere">{intl.formatMessage(messages.title)}</h1>
        <p className="text-fg-muted max-w-screen-sm wrap-anywhere">{intl.formatMessage(messages.text)}</p>
        <div className="flex flex-wrap items-center justify-center gap-3">
          <Button asChild>
            <a href="#flows">{intl.formatMessage(messages.home)}</a>
          </Button>
          <Link href="#back" className="inline-flex min-h-target items-center">
            {intl.formatMessage(messages.back)}
          </Link>
        </div>
      </main>
    </div>
  );
}
