// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { useId } from 'react';
import { useIntl } from 'react-intl';

import { Button } from '../components/button/Button';
import { StatusBadge } from '../components/status-badge/StatusBadge';
import { cn } from '../lib/cn';

import { canvasMessages } from './messages';
import { readingOrder, type Graph, type GraphEdge, type GraphNode } from './model';

export interface CanvasOutlineProps {
  graph: Graph;
  /** Shows the actions that change the flow (remove a connection, delete a node). */
  editable?: boolean;
  /** Moves keyboard focus to the node on the canvas. */
  onFocusNode?: (id: string) => void;
  onDisconnect?: (edgeId: string) => void;
  onDeleteNode?: (id: string) => void;
  /** Heading level of the title, to keep the page outline correct. */
  headingLevel?: 2 | 3 | 4;
  className?: string;
}

/** Outline buttons carry the names of nodes, so their labels wrap instead of running out of the panel. */
const wrapLabel = 'h-auto py-2 text-left whitespace-normal';

function portName(node: GraphNode | undefined, side: 'inputs' | 'outputs', id: string): string {
  return node?.[side].find((port) => port.id === id)?.label ?? id;
}

/**
 * The flow as text: every node with its state, and under it every connection it
 * has. This is how a screen reader user reads the graph, and, with the actions,
 * a way to change it without a pointer. It describes the same data the canvas
 * draws, so the two never disagree.
 */
export function CanvasOutline({ graph, editable = false, onFocusNode, onDisconnect, onDeleteNode, headingLevel = 2, className }: CanvasOutlineProps) {
  const intl = useIntl();
  const titleId = useId();
  const Heading = `h${headingLevel}` as const;
  const byId = new Map(graph.nodes.map((node) => [node.id, node]));
  const nodes = readingOrder(graph);

  const connectionsOf = (node: GraphNode) =>
    graph.edges.filter((edge) => edge.source === node.id || edge.target === node.id).map((edge) => ({ edge, outgoing: edge.source === node.id }));

  const describe = (node: GraphNode, edge: GraphEdge, outgoing: boolean) => {
    const other = byId.get(outgoing ? edge.target : edge.source);
    return {
      otherName: other?.name ?? (outgoing ? edge.target : edge.source),
      text: intl.formatMessage(canvasMessages.outlineConnection, {
        direction: outgoing ? 'out' : 'in',
        port: portName(node, outgoing ? 'outputs' : 'inputs', outgoing ? edge.sourceOutput : edge.targetInput),
        other: other?.name ?? (outgoing ? edge.target : edge.source),
        otherPort: portName(other, outgoing ? 'inputs' : 'outputs', outgoing ? edge.targetInput : edge.sourceOutput),
        hasItems: typeof edge.items === 'number' ? 'yes' : 'no',
        items: edge.items ?? 0,
      }),
    };
  };

  return (
    <section aria-labelledby={titleId} data-canvas-outline className={cn('bg-surface text-fg flex flex-col gap-3', className)}>
      <div className="flex flex-col gap-1">
        <Heading id={titleId} className="text-md font-semibold wrap-anywhere">
          {intl.formatMessage(canvasMessages.outlineTitle)}
        </Heading>
        <p className="text-fg-muted text-sm wrap-anywhere">
          {intl.formatMessage(canvasMessages.outlineSummary, { nodes: graph.nodes.length, edges: graph.edges.length })}
        </p>
      </div>
      {nodes.length === 0 ? (
        <p className="text-fg-muted text-sm wrap-anywhere">{intl.formatMessage(canvasMessages.outlineEmpty)}</p>
      ) : (
        <ol className="flex flex-col gap-3">
          {nodes.map((node) => {
            const connections = connectionsOf(node);
            return (
              <li key={node.id} data-outline-node={node.id} className="border-border flex flex-col gap-2 border-t pt-3">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="font-semibold wrap-anywhere">{node.name}</span>
                  {node.kind === 'control' ? (
                    <span className="text-fg-muted text-sm">{intl.formatMessage(canvasMessages.controlType, { type: node.controlType ?? 'other' })}</span>
                  ) : null}
                  {node.status ? <StatusBadge status={node.status} /> : null}
                </div>
                {node.summary ? <p className="text-fg-muted text-sm wrap-anywhere">{node.summary}</p> : null}
                {connections.length === 0 ? (
                  <p className="text-fg-muted text-sm wrap-anywhere">{intl.formatMessage(canvasMessages.outlineNoConnections)}</p>
                ) : (
                  <ul className="flex flex-col gap-1">
                    {connections.map(({ edge, outgoing }) => {
                      const { text, otherName } = describe(node, edge, outgoing);
                      return (
                        <li key={`${edge.id}-${outgoing ? 'out' : 'in'}`} className="flex flex-wrap items-center gap-2 text-sm">
                          <span className="wrap-anywhere">{text}</span>
                          {editable && outgoing ? (
                            <Button variant="ghost" className={wrapLabel} onClick={() => onDisconnect?.(edge.id)}>
                              {intl.formatMessage(canvasMessages.outlineDisconnect, { name: node.name, other: otherName })}
                            </Button>
                          ) : null}
                        </li>
                      );
                    })}
                  </ul>
                )}
                <div className="flex flex-wrap gap-2">
                  <Button variant="secondary" className={wrapLabel} onClick={() => onFocusNode?.(node.id)}>
                    {intl.formatMessage(canvasMessages.outlineFocus, { name: node.name })}
                  </Button>
                  {editable ? (
                    <Button variant="ghost" className={wrapLabel} onClick={() => onDeleteNode?.(node.id)}>
                      {intl.formatMessage(canvasMessages.outlineDelete, { name: node.name })}
                    </Button>
                  ) : null}
                </div>
              </li>
            );
          })}
        </ol>
      )}
    </section>
  );
}
