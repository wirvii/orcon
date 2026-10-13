// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Brain, Database, FileJson, Plus, Search, Wrench, type LucideIcon } from 'lucide-react';
import { useIntl } from 'react-intl';

import { cn } from '../../lib/cn';
import { canvasMessages } from '../messages';
import type { AiBlockKind } from '../model';

const icons: Record<AiBlockKind, LucideIcon> = {
  model: Brain,
  memory: Database,
  tools: Wrench,
  parser: FileJson,
  retrieval: Search,
};

const labels = {
  model: canvasMessages.aiModel,
  memory: canvasMessages.aiMemory,
  tools: canvasMessages.aiTools,
  parser: canvasMessages.aiParser,
  retrieval: canvasMessages.aiRetrieval,
} as const;

export interface AiBlockProps {
  kind: AiBlockKind;
  /** The block is configured. An unset block is a dashed circle with a plus. */
  set?: boolean;
  /** What is configured ("Chat model", "Window memory"), for the accessible name. */
  detail?: string;
  tabbable?: boolean;
  /** A view-only canvas shows the block without a button, so nothing small is left to tap. Default true. */
  interactive?: boolean;
  /** Opens the Agent tab of the node details view. */
  onActivate?: () => void;
  className?: string;
}

/**
 * A configuration block of an agent node: a 48 px circle under the node with its name in mono text.
 * A block is not a node: it is not selectable on its own, has no ports and does not execute.
 * Activating it opens the Agent tab of the node details view.
 */
export function AiBlock({ kind, set = true, detail, tabbable = true, interactive = true, onActivate, className }: AiBlockProps) {
  const intl = useIntl();
  const Icon = set ? icons[kind] : Plus;
  const block = intl.formatMessage(labels[kind]);
  const name = set
    ? intl.formatMessage(canvasMessages.aiBlockSet, { block, detail: detail ?? block })
    : intl.formatMessage(canvasMessages.aiBlockUnset, { block });
  return (
    <div className={cn('flex flex-col items-center gap-1', className)} data-ai-block={kind}>
      {interactive ? (
        <button
          type="button"
          aria-label={name}
          tabIndex={tabbable ? 0 : -1}
          onClick={(event) => {
            event.stopPropagation();
            onActivate?.();
          }}
          className={cn(
            'nodrag nopan grid size-canvas-ai-block place-items-center rounded-full border-2 p-0',
            set
              ? 'bg-canvas-node-bg border-canvas-node-border text-fg shadow-sm hover:border-canvas-edge'
              : 'bg-canvas-node-bg border-canvas-edge text-fg-muted border-dashed hover:border-action-primary hover:text-action-primary',
          )}
        >
          <Icon aria-hidden className="size-icon-24" />
        </button>
      ) : (
        <div
          role="img"
          aria-label={name}
          className={cn(
            'grid size-canvas-ai-block place-items-center rounded-full border-2 p-0',
            set
              ? 'bg-canvas-node-bg border-canvas-node-border text-fg shadow-sm hover:border-canvas-edge'
              : 'bg-canvas-node-bg border-canvas-edge text-fg-muted border-dashed hover:border-action-primary hover:text-action-primary',
          )}
        >
          <Icon aria-hidden className="size-icon-24" />
        </div>
      )}
      <span aria-hidden className="text-fg-muted font-mono text-xs font-semibold">
        {block}
      </span>
    </div>
  );
}
