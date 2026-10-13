// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { IntlShape } from 'react-intl';

import { statusMap } from '../components/status-badge/status-map';

import { canvasMessages } from './messages';
import type { GraphEdge, GraphNode } from './model';

/** "<name>, <operation>. Status: <status>[, pinned][, disabled]. <n> inputs, <m> outputs." */
export function nodeAriaLabel(intl: IntlShape, node: GraphNode): string {
  const status = node.status ? intl.formatMessage(statusMap[node.status].label) : intl.formatMessage(canvasMessages.statusDesign);
  return intl.formatMessage(canvasMessages.nodeLabel, {
    name: node.name,
    hasSubtitle: node.subtitle ? 'yes' : 'no',
    subtitle: node.subtitle ?? '',
    status,
    pinned: node.pinned ? 'yes' : 'no',
    disabled: node.disabled ? 'yes' : 'no',
    inputs: node.inputs.length,
    outputs: node.outputs.length,
  });
}

/** The label a port shows. Several ports on one side always show one, falling back to the port id. */
export function portLabel(ports: readonly { id: string; label?: string | undefined }[], port: { id: string; label?: string | undefined }): string | undefined {
  return port.label ?? (ports.length > 1 ? port.id : undefined);
}

/** "<source> (<output>) to <target>: <count>, <state>" (the last part only when an execution is shown). */
export function edgeAriaLabel(intl: IntlShape, edge: GraphEdge, source: GraphNode | undefined, target: GraphNode | undefined): string {
  const output = source?.outputs.find((port) => port.id === edge.sourceOutput);
  const items =
    edge.state === 'uncertain'
      ? intl.formatMessage(canvasMessages.edgeUnknown)
      : intl.formatMessage(canvasMessages.edgeItems, { count: edge.items ?? 0 });
  return intl.formatMessage(canvasMessages.edgeLabel, {
    source: source?.name ?? edge.source,
    output: output?.label ?? edge.sourceOutput,
    target: target?.name ?? edge.target,
    hasState: edge.state ? 'yes' : 'no',
    items,
    state: edge.state ? intl.formatMessage(canvasMessages.edgeState, { state: edge.state }) : '',
    loop: edge.loopBack ? 'yes' : 'no',
  });
}
