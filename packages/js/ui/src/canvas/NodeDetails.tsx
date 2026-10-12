// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { useIntl } from 'react-intl';

import { Link } from '../components/link/Link';
import { Sheet } from '../components/sheet/Sheet';
import { StatusBadge } from '../components/status-badge/StatusBadge';

import { canvasMessages } from './messages';
import { itemCounts, type Graph, type GraphNode } from './model';

export interface NodeDetailsProps {
  graph: Graph;
  node: GraphNode | undefined;
  onClose: () => void;
  /** Link to the execution detail of this node. Omit when no execution is being shown. */
  executionHref?: string;
  /** `bottom` on phones, `responsive` (side panel from md) elsewhere. */
  side?: 'bottom' | 'responsive';
}

/**
 * What a person sees after tapping a node on a phone, where the canvas is read
 * only: the state, the item counts and a link to the execution detail.
 */
export function NodeDetails({ graph, node, onClose, executionHref, side = 'bottom' }: NodeDetailsProps) {
  const intl = useIntl();
  const counts = node ? itemCounts(graph, node.id) : { received: 0, sent: 0 };
  return (
    <Sheet open={node !== undefined} onOpenChange={(open) => (open ? undefined : onClose())} title={node?.name ?? ''} description={node?.summary} side={side}>
      {node ? (
        <div className="flex flex-col gap-3" data-node-details={node.id}>
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-fg-muted text-sm">{intl.formatMessage(canvasMessages.detailsStatus)}</span>
            {node.status ? <StatusBadge status={node.status} /> : <span className="text-fg-muted text-sm">{intl.formatMessage(canvasMessages.detailsNoStatus)}</span>}
          </div>
          {node.uncertainCount ? (
            <p className="text-status-attention text-sm font-medium">{intl.formatMessage(canvasMessages.uncertainItems, { count: node.uncertainCount })}</p>
          ) : null}
          <ul className="flex flex-col gap-1 text-sm">
            <li>{intl.formatMessage(canvasMessages.detailsReceived, { count: counts.received })}</li>
            <li>{intl.formatMessage(canvasMessages.detailsSent, { count: counts.sent })}</li>
          </ul>
          {executionHref ? (
            <Link href={executionHref} className="self-start">
              {intl.formatMessage(canvasMessages.detailsExecution)}
            </Link>
          ) : null}
        </div>
      ) : null}
    </Sheet>
  );
}
