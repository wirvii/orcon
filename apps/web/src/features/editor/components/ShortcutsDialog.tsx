// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Dialog, KbdCombination } from '@orcon/ui';
import { useIntl } from 'react-intl';

import { AVAILABLE_ENTRIES, chordKeys, type Platform } from '../model/keymap';

import { actionMessages, editorMessages } from './messages';

/** The keyboard shortcuts of this release, listed from the same keymap the handlers use (SPEC-014 R-ED-KEY-01). */
export function ShortcutsDialog({ open, onOpenChange, platform }: { open: boolean; onOpenChange: (open: boolean) => void; platform: Platform }) {
  const intl = useIntl();
  return (
    <Dialog open={open} onOpenChange={onOpenChange} title={intl.formatMessage(editorMessages.shortcutsTitle)} description={intl.formatMessage(editorMessages.shortcutsIntro)}>
      <dl className="flex flex-col gap-2">
        {AVAILABLE_ENTRIES.map((entry) => (
          <div key={entry.action} className="border-border flex flex-wrap items-center justify-between gap-2 border-b pb-2">
            <dt className="wrap-anywhere">{intl.formatMessage(actionMessages[entry.action])}</dt>
            <dd className="flex flex-wrap items-center gap-2">
              {(entry.display ?? entry.chords).map((chord) => (
                <KbdCombination key={chord} keys={chordKeys(chord, platform)} />
              ))}
            </dd>
          </div>
        ))}
      </dl>
    </Dialog>
  );
}
