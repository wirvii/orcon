// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Banner, Button } from '@orcon/ui';
import { useEffect, useSyncExternalStore } from 'react';
import { useIntl } from 'react-intl';

import { editorMessages } from './messages';
import { useNoticeChannel } from './notice-channel';

/** How long a confirmation stays; a refusal stays until the next one or until it is dismissed. */
const INFO_MS = 5_000;

/**
 * Shows the latest notice of the editor as a banner over the top of the canvas, so the canvas does
 * not move when a notice comes and goes. It is announced politely like every banner that is not an error.
 */
export function NoticeBar() {
  const intl = useIntl();
  const channel = useNoticeChannel();
  const notice = useSyncExternalStore(channel.subscribe, channel.current);

  useEffect(() => {
    if (notice?.tone !== 'info') return;
    const timer = setTimeout(channel.dismiss, INFO_MS);
    return () => {
      clearTimeout(timer);
    };
  }, [notice, channel]);

  if (!notice) return null;
  return (
    <div className="pointer-events-none absolute inset-x-0 top-3 z-sticky flex justify-center px-4">
      <Banner
        key={notice.id}
        className="pointer-events-auto w-full max-w-screen-sm shadow-md"
        tone={notice.tone === 'warning' ? 'warning' : 'info'}
        title={notice.message}
        action={
          <Button variant="ghost" onClick={channel.dismiss}>
            {intl.formatMessage(editorMessages.dismiss)}
          </Button>
        }
      />
    </div>
  );
}
