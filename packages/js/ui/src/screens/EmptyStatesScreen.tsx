// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { defineMessages, useIntl } from 'react-intl';

import { Button } from '../components/button/Button';
import { EmptyState } from '../components/empty-state/EmptyState';
import { Mascot } from '../components/mascot/Mascot';

import { PageHeader } from './PageHeader';
import { ProductShell } from './ProductShell';

const messages = defineMessages({
  title: { id: 'screen.empty.title', defaultMessage: 'Empty states', description: 'Heading of the template page that shows the empty states of the product.' },
  description: {
    id: 'screen.empty.description',
    defaultMessage: 'What a screen says when there is nothing to show yet. Each one says what to do next.',
    description: 'One line under the heading of the empty states template.',
  },
  firstTitle: { id: 'screen.empty.first.title', defaultMessage: 'No flows yet', description: 'Title of the empty state of a list the person has not filled yet.' },
  firstText: {
    id: 'screen.empty.first.text',
    defaultMessage: 'Create a flow to start automating. It stays off until you turn it on.',
    description: 'Text of the first-time empty state: what to do next.',
  },
  firstAction: { id: 'screen.empty.first.action', defaultMessage: 'New flow', description: 'Button of the first-time empty state.' },
  searchTitle: { id: 'screen.empty.search.title', defaultMessage: 'Nothing matches your search', description: 'Title of the empty state of a search without results.' },
  searchText: {
    id: 'screen.empty.search.text',
    defaultMessage: 'Try another word, or clear the filters to see everything.',
    description: 'Text of the empty state of a search without results: what to try.',
  },
  searchAction: { id: 'screen.empty.search.action', defaultMessage: 'Clear filters', description: 'Button of the empty state of a search without results.' },
  inboxTitle: { id: 'screen.empty.inbox.title', defaultMessage: 'Nothing is waiting for you', description: 'Title of the empty state of the attention inbox.' },
  inboxText: {
    id: 'screen.empty.inbox.text',
    defaultMessage: 'When a flow needs a decision, it appears here.',
    description: 'Text of the empty state of the attention inbox.',
  },
});

/**
 * S8 (part 1): the empty states. The happy mascot appears only where nothing
 * has been created yet; a search with no results and a dense list use text only.
 */
export function EmptyStatesScreen() {
  const intl = useIntl();
  return (
    <ProductShell current="flows">
      <PageHeader title={intl.formatMessage(messages.title)} description={intl.formatMessage(messages.description)} />
      <div className="grid gap-4 lg:grid-cols-3">
        <div className="bg-surface border-border rounded-lg border">
          <EmptyState
            title={intl.formatMessage(messages.firstTitle)}
            description={intl.formatMessage(messages.firstText)}
            mascot={<Mascot pose="happy" />}
            headingLevel={2}
            action={<Button>{intl.formatMessage(messages.firstAction)}</Button>}
          />
        </div>
        <div className="bg-surface border-border rounded-lg border">
          <EmptyState
            title={intl.formatMessage(messages.searchTitle)}
            description={intl.formatMessage(messages.searchText)}
            headingLevel={2}
            action={<Button variant="secondary">{intl.formatMessage(messages.searchAction)}</Button>}
          />
        </div>
        <div className="bg-surface border-border rounded-lg border">
          <EmptyState title={intl.formatMessage(messages.inboxTitle)} description={intl.formatMessage(messages.inboxText)} headingLevel={2} />
        </div>
      </div>
    </ProductShell>
  );
}
