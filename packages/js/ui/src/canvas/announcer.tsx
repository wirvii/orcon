// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';

/** At most one announcement every five seconds. Anything announced in between is joined into it. */
export const ANNOUNCE_INTERVAL_MS = 5000;

export interface Announcer {
  announce: (message: string) => void;
  /** Render this once. It is an invisible polite live region that never takes focus. */
  region: ReactNode;
}

/**
 * Polite, coalesced announcements for assistive technology. The first message is
 * spoken at once; messages that arrive while the interval runs are joined and
 * spoken together when it ends, so a burst of state changes is one announcement.
 */
export function useAnnouncer(intervalMs = ANNOUNCE_INTERVAL_MS): Announcer {
  const [message, setMessage] = useState('');
  const queue = useRef<string[]>([]);
  const lastFlush = useRef<number | undefined>(undefined);
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const toggle = useRef(false);

  const flush = useCallback(() => {
    timer.current = undefined;
    if (queue.current.length === 0) return;
    toggle.current = !toggle.current;
    // A trailing no-break space changes the text so the same sentence is announced again.
    setMessage(`${queue.current.join('. ')}${toggle.current ? ' ' : ''}`);
    queue.current = [];
    lastFlush.current = Date.now();
  }, []);

  const announce = useCallback(
    (text: string) => {
      queue.current.push(text);
      if (timer.current !== undefined) return;
      const wait = lastFlush.current === undefined ? 0 : Math.max(0, lastFlush.current + intervalMs - Date.now());
      timer.current = setTimeout(flush, wait);
    },
    [flush, intervalMs],
  );

  useEffect(
    () => () => {
      if (timer.current !== undefined) clearTimeout(timer.current);
    },
    [],
  );

  const region = (
    <div role="status" aria-live="polite" aria-atomic="true" className="sr-only" data-canvas-announcer>
      {message}
    </div>
  );
  return { announce, region };
}
