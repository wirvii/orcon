// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { parseFlowDocument, type FlowDocument } from './flow-document';

/** The id of the sample flow the editor opens until it loads flows from the server. */
export const EXAMPLE_FLOW_ID = '01930000-0000-7000-8000-00000000ed5a';

/** The id the performance build opens the 500-node reference flow under (`/flows/<id>`). */
export const PERF_FLOW_ID = '01930000-0000-7000-8000-000000000500';

/**
 * A sample flow in the `orcon/flow@1` format: a webhook receives an order, an If node checks it,
 * an HTTP request looks the stock up and a second HTTP request posts to a chat webhook. It is the
 * flow the preview of the editor opens (no request is made to read it).
 */
const example = {
  format: 'orcon/flow@1',
  id: EXAMPLE_FLOW_ID,
  name: 'Order notifications',
  description: 'Receives an order, checks it, looks the stock up and tells the team in Slack.',
  settings: {},
  variables: [],
  nodes: [
    { id: 'webhook', name: 'Webhook', element: 'startEvent', type: 'orcon.start@1', parameters: { method: 'POST', path: '/orders' } },
    {
      id: 'validate',
      name: 'Validate order',
      element: 'exclusiveGateway',
      type: 'orcon.if@1',
      parameters: { condition: '={{ $json.total > 0 }}' },
      routing: 'EXCLUSIVE',
    },
    {
      id: 'inventory',
      name: 'HTTP Request',
      element: 'serviceTask',
      type: 'orcon.http@1',
      parameters: { method: 'GET', url: '=https://inventory.example.test/items/{{ $json.sku }}' },
    },
    {
      id: 'slack',
      name: 'Slack',
      element: 'serviceTask',
      type: 'orcon.http@1',
      parameters: {
        method: 'POST',
        url: 'https://hooks.slack.example.test/services/orders',
        body: '={{ { text: "Order " + $(\'Webhook\').item.json.id + ": " + $(\'HTTP Request\').item.json.stock + " in stock" } }}',
      },
    },
    { id: 'rejected', name: 'Ignore invalid order', element: 'endEvent', type: 'orcon.end@1', parameters: {} },
    { id: 'done', name: 'Done', element: 'endEvent', type: 'orcon.end@1', parameters: {} },
  ],
  edges: [
    { id: 'e1', from: { node: 'webhook', port: 'main' }, to: { node: 'validate' }, kind: 'SEQUENCE' },
    { id: 'e2', from: { node: 'validate', port: 'true' }, to: { node: 'inventory' }, kind: 'SEQUENCE' },
    { id: 'e3', from: { node: 'validate', port: 'false' }, to: { node: 'rejected' }, kind: 'SEQUENCE' },
    { id: 'e4', from: { node: 'inventory', port: 'main' }, to: { node: 'slack' }, kind: 'SEQUENCE' },
    { id: 'e5', from: { node: 'slack', port: 'main' }, to: { node: 'done' }, kind: 'SEQUENCE' },
  ],
  editor: {
    nodes: {
      webhook: { x: 40, y: 160 },
      validate: { x: 240, y: 160 },
      inventory: { x: 440, y: 60 },
      slack: { x: 640, y: 60 },
      done: { x: 840, y: 60 },
      rejected: { x: 440, y: 260 },
    },
    stickyNotes: [
      {
        id: 'note-1',
        x: 40,
        y: 340,
        w: 260,
        h: 120,
        color: 2,
        title: 'Order intake',
        body: 'Checks the order, looks the stock up, then tells the team in Slack.',
      },
    ],
  },
};

/** A fresh copy of the sample flow. */
export function exampleFlowDocument(): FlowDocument {
  const parsed = parseFlowDocument(example);
  if (!parsed.ok) throw new Error(`The sample flow is not a valid flow document: ${parsed.message}`);
  return parsed.document;
}
