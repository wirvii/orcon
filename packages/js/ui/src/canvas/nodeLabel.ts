// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { IntlShape } from 'react-intl';

import { statusMap } from '../components/status-badge/status-map';

import type { GraphNode } from './model';
import { canvasMessages } from './messages';

/** "<kind> node <name>, <summary>, status <status>, <n> inputs, <m> outputs". */
export function nodeAriaLabel(intl: IntlShape, node: GraphNode): string {
  const status = node.status ? intl.formatMessage(statusMap[node.status].label) : '';
  return intl.formatMessage(canvasMessages.nodeLabel, {
    kind: node.kind,
    name: node.name,
    hasSummary: node.summary ? 'yes' : 'no',
    summary: node.summary ?? '',
    hasStatus: status ? 'yes' : 'no',
    status,
    inputs: node.inputs.length,
    outputs: node.outputs.length,
  });
}

/** The label a port shows. Several ports on one side always show one, falling back to the port id. */
export function portLabel(ports: readonly { id: string; label?: string }[], port: { id: string; label?: string }): string | undefined {
  return port.label ?? (ports.length > 1 ? port.id : undefined);
}
