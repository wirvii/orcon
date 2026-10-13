// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Ban, CircleHelp, Minus, PenLine, Pin, Plus, ShieldQuestion } from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
import { useIntl } from 'react-intl';

import { statusMap, type StatusFamily, type StatusKey } from '../../components/status-badge/status-map';
import { cn } from '../../lib/cn';
import { canvasMessages } from '../messages';
import { familyClasses } from '../state';

export type ChipKind = 'pinned' | 'disabled' | 'not-supported' | 'added' | 'changed' | 'removed' | 'uncertain-items';

export interface NodeStatusChipProps {
  /** An execution state. The chip shows its icon and label. */
  status?: StatusKey;
  /** A short error code shown with Failed ("Failed · 401"). */
  code?: string;
  /** A chip that is not an execution state: pinned data, disabled, an unsupported node, a compare mark. */
  kind?: ChipKind;
  /** Items that are UNCERTAIN, for kind "uncertain-items". */
  count?: number;
  className?: string;
}

interface ChipLook {
  family: StatusFamily;
  icon: LucideIcon;
  dashed?: boolean;
}

const kindLooks: Record<ChipKind, ChipLook> = {
  pinned: { family: 'neutral', icon: Pin },
  disabled: { family: 'neutral', icon: Ban, dashed: true },
  'not-supported': { family: 'warning', icon: CircleHelp },
  added: { family: 'success', icon: Plus },
  changed: { family: 'warning', icon: PenLine },
  removed: { family: 'danger', icon: Minus },
  'uncertain-items': { family: 'attention', icon: ShieldQuestion },
};

/**
 * A small pill under a node name: icon + text, in the family of the state. Succeeded has no chip
 * (the quiet state); every other state or mark has one, so color is never the only signal.
 */
export function NodeStatusChip({ status, code, kind, count = 0, className }: NodeStatusChipProps) {
  const intl = useIntl();
  let look: ChipLook;
  let text: string;
  let spins = false;
  if (kind) {
    look = kindLooks[kind];
    text = {
      pinned: intl.formatMessage(canvasMessages.chipPinned),
      disabled: intl.formatMessage(canvasMessages.chipDisabled),
      'not-supported': intl.formatMessage(canvasMessages.chipNotSupported),
      added: intl.formatMessage(canvasMessages.chipAdded),
      changed: intl.formatMessage(canvasMessages.chipChanged),
      removed: intl.formatMessage(canvasMessages.chipRemoved),
      'uncertain-items': intl.formatMessage(canvasMessages.chipUncertainItems, { count }),
    }[kind];
  } else if (status) {
    const entry = statusMap[status];
    look = { family: entry.family, icon: entry.icon };
    spins = Boolean(entry.spins);
    text =
      (status === 'FAILED' || status === 'ERROR_FINAL') && code
        ? intl.formatMessage(canvasMessages.chipFailedCode, { code })
        : intl.formatMessage(entry.label);
  } else {
    return null;
  }
  const family = familyClasses[look.family];
  const Icon = look.icon;
  return (
    <span
      data-node-chip={kind ?? status}
      className={cn(
        'inline-flex min-h-5 max-w-full items-center gap-1 rounded-full border px-2 text-xs font-semibold whitespace-nowrap',
        family.bg,
        family.text,
        family.border,
        look.dashed && 'border-dashed',
        className,
      )}
    >
      <Icon aria-hidden className={cn('size-3 shrink-0', spins && 'motion-safe:animate-loop-spin')} />
      <span>{text}</span>
    </span>
  );
}
