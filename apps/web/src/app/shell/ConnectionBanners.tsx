// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Banner } from '@orcon/ui';
import { onlineManager } from '@tanstack/react-query';
import { useSyncExternalStore } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { useLiveEventsState } from '../live-events/LiveEventsProvider';

const messages = defineMessages({
  offlineTitle: {
    id: 'app.offline.title',
    defaultMessage: 'You are offline',
    description: 'Title of the banner shown while the browser has no network connection.',
  },
  offlineText: {
    id: 'app.offline.text',
    defaultMessage: 'What you see may be out of date. Requests resume when the connection returns.',
    description: 'Explains the offline banner: data may be stale and requests wait for the network.',
  },
  pausedTitle: {
    id: 'app.live.paused.title',
    defaultMessage: 'Live updates are paused',
    description: 'Title of the banner shown while the connection that delivers live changes is down.',
  },
  pausedText: {
    id: 'app.live.paused.text',
    defaultMessage: 'The page reconnects by itself and refreshes what it shows.',
    description: 'Explains the banner about paused live updates: the page reconnects and refreshes.',
  },
});

const subscribeOnline = (onChange: () => void) => onlineManager.subscribe(onChange);
const isOnline = () => onlineManager.isOnline();

/** The banners about the connection, shown above the page of every shell. */
export function ConnectionBanners() {
  const intl = useIntl();
  const liveState = useLiveEventsState();
  const online = useSyncExternalStore(subscribeOnline, isOnline);
  return (
    <>
      {online ? null : (
        <Banner tone="warning" title={intl.formatMessage(messages.offlineTitle)}>
          {intl.formatMessage(messages.offlineText)}
        </Banner>
      )}
      {liveState === 'paused' ? (
        <Banner tone="warning" title={intl.formatMessage(messages.pausedTitle)}>
          {intl.formatMessage(messages.pausedText)}
        </Banner>
      ) : null}
    </>
  );
}
