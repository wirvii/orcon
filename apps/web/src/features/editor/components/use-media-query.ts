// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { useSyncExternalStore } from 'react';

/** The md breakpoint (768 px): from it the editor edits, below it the canvas is read-only (SPEC-014 R-ED-MOB-01). */
export const EDITING_QUERY = '(min-width: 48rem)';

/** Whether a media query matches now, and again when it changes. Where there is no `matchMedia` it reports false. */
export function useMediaQuery(query: string): boolean {
  return useSyncExternalStore(
    (notify) => {
      if (typeof window.matchMedia !== 'function') return () => undefined;
      const list = window.matchMedia(query);
      list.addEventListener('change', notify);
      return () => {
        list.removeEventListener('change', notify);
      };
    },
    () => typeof window.matchMedia === 'function' && window.matchMedia(query).matches,
    () => false,
  );
}
