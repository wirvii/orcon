// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Spinner } from '@orcon/ui';
import { CircleAlert, CircleCheck, CircleDot, TriangleAlert, type LucideIcon } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import { useIntl, type IntlShape, type MessageDescriptor } from 'react-intl';

import type { SaveStatus } from '../model/autosave';

import { editorMessages } from './messages';

const looks: Record<SaveStatus, { icon: LucideIcon | undefined; text: MessageDescriptor; className: string }> = {
  saved: { icon: CircleCheck, text: editorMessages.saveSaved, className: 'text-fg-muted' },
  saving: { icon: undefined, text: editorMessages.saveSaving, className: 'text-fg-muted' },
  unsaved: { icon: CircleDot, text: editorMessages.saveUnsaved, className: 'text-fg-muted' },
  retrying: { icon: TriangleAlert, text: editorMessages.saveRetrying, className: 'text-status-warning' },
  conflict: { icon: CircleAlert, text: editorMessages.saveConflict, className: 'text-status-danger' },
};

/** How often the age of the last save is written again, so the text stays true without a timer per second. */
const TICK_MS = 5_000;

/** "now", "4 s ago", "3 min ago", "2 h ago" in the person's language, in steps coarse enough not to flicker. */
export function formatAge(intl: IntlShape, milliseconds: number): string {
  const seconds = Math.max(0, Math.round(milliseconds / 1000));
  if (seconds < 5) return intl.formatRelativeTime(0, 'second', { numeric: 'auto', style: 'narrow' });
  if (seconds < 60) return intl.formatRelativeTime(-Math.floor(seconds / 5) * 5, 'second', { style: 'narrow' });
  if (seconds < 3600) return intl.formatRelativeTime(-Math.floor(seconds / 60), 'minute', { style: 'narrow' });
  return intl.formatRelativeTime(-Math.floor(seconds / 3600), 'hour', { style: 'narrow' });
}

/**
 * The state of the draft as an icon and text, never color alone (SPEC-014 R-ED-DRAFT-01, R-ED-A11Y-03). After a
 * save made in this tab it says when: "Saved · 4 s ago".
 */
export function SaveIndicator({ status }: { status: SaveStatus }) {
  const intl = useIntl();
  const { icon: Icon, text, className } = looks[status];
  const [savedAt, setSavedAt] = useState<number | undefined>(undefined);
  const [now, setNow] = useState(() => Date.now());
  const before = useRef<SaveStatus>(status);

  useEffect(() => {
    // Only a save in this tab has a time: the first "Saved" of a freshly opened flow does not.
    if (status === 'saved' && before.current !== 'saved') {
      const at = Date.now();
      setSavedAt(at);
      setNow(at);
    }
    before.current = status;
  }, [status]);

  useEffect(() => {
    if (status !== 'saved' || savedAt === undefined) return undefined;
    const timer = setInterval(() => {
      setNow(Date.now());
    }, TICK_MS);
    return () => {
      clearInterval(timer);
    };
  }, [status, savedAt]);

  const label =
    status === 'saved' && savedAt !== undefined
      ? intl.formatMessage(editorMessages.saveSavedAt, { age: formatAge(intl, now - savedAt) })
      : intl.formatMessage(text);

  return (
    <span role="status" data-save-state={status} className={`inline-flex items-center gap-1 text-sm whitespace-nowrap ${className}`}>
      {Icon ? <Icon aria-hidden className="size-icon-16" /> : <Spinner size={16} decorative />}
      <span className="sr-only">{intl.formatMessage(editorMessages.saveLabel)}: </span>
      <span>{label}</span>
    </span>
  );
}
