// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createContext, useContext } from 'react';

export interface Notice {
  /** Changes with every notice, so the same sentence twice is announced twice. */
  readonly id: number;
  /** `info` confirms; `warning` says something was refused or is not available. */
  readonly tone: 'info' | 'warning';
  readonly message: string;
}

export interface NoticeChannel {
  notify: (tone: Notice['tone'], message: string) => void;
  dismiss: () => void;
  subscribe: (listener: () => void) => () => void;
  current: () => Notice | undefined;
}

/**
 * Where the editor tells a person what it just did or refused ("Pasted 3 items", "That connection
 * would make a loop"). One channel per open editor; the notice bar shows the latest notice. The
 * channel is plain state outside React so any handler can use it, and the context carries only the
 * channel itself, which never changes (orcon-web ST-11).
 */
export function createNoticeChannel(): NoticeChannel {
  let notice: Notice | undefined;
  let counter = 0;
  const listeners = new Set<() => void>();
  const emit = () => {
    listeners.forEach((listener) => {
      listener();
    });
  };
  return {
    notify: (tone, message) => {
      counter += 1;
      notice = { id: counter, tone, message };
      emit();
    },
    dismiss: () => {
      notice = undefined;
      emit();
    },
    subscribe: (listener) => {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    current: () => notice,
  };
}

export const NoticeContext = createContext<NoticeChannel | undefined>(undefined);

export function useNoticeChannel(): NoticeChannel {
  const channel = useContext(NoticeContext);
  if (!channel) throw new Error('Notices are only available inside the flow editor.');
  return channel;
}
