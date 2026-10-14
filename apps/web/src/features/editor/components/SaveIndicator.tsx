// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Spinner } from '@orcon/ui';
import { CircleAlert, CircleCheck, CircleDot, TriangleAlert, type LucideIcon } from 'lucide-react';
import { useIntl, type MessageDescriptor } from 'react-intl';

import type { SaveStatus } from '../model/autosave';

import { editorMessages } from './messages';

const looks: Record<SaveStatus, { icon: LucideIcon | undefined; text: MessageDescriptor; className: string }> = {
  saved: { icon: CircleCheck, text: editorMessages.saveSaved, className: 'text-fg-muted' },
  saving: { icon: undefined, text: editorMessages.saveSaving, className: 'text-fg-muted' },
  unsaved: { icon: CircleDot, text: editorMessages.saveUnsaved, className: 'text-fg-muted' },
  retrying: { icon: TriangleAlert, text: editorMessages.saveRetrying, className: 'text-status-warning' },
  conflict: { icon: CircleAlert, text: editorMessages.saveConflict, className: 'text-status-danger' },
};

/** The state of the draft as an icon and text, never color alone (SPEC-014 R-ED-DRAFT-01, R-ED-A11Y-03). */
export function SaveIndicator({ status }: { status: SaveStatus }) {
  const intl = useIntl();
  const { icon: Icon, text, className } = looks[status];
  return (
    <span role="status" data-save-state={status} className={`inline-flex items-center gap-1 text-sm ${className}`}>
      {Icon ? <Icon aria-hidden className="size-icon-16" /> : <Spinner size={16} decorative />}
      <span className="sr-only">{intl.formatMessage(editorMessages.saveLabel)}: </span>
      <span>{intl.formatMessage(text)}</span>
    </span>
  );
}
