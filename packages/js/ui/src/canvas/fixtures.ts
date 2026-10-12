// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Example flows for stories, tests and the screen templates. No request is made: the
// logo is a 2x2 image embedded in the file.

import type { CatalogItem, Graph, GraphNode } from './model';

export const placeholderLogo =
  'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAIAAAACCAIAAAD91JpzAAAAEElEQVR4nGNIKekGIgYIBQAmRgWNPOJSIgAAAABJRU5ErkJggg==';

const main = [{ id: 'main' }];

export const exampleNodes = {
  trigger: {
    id: 'order-received',
    kind: 'trigger',
    name: 'Order received',
    summary: 'Starts the flow when the shop sends a new order.',
    connector: { src: placeholderLogo, name: 'Shop webhook' },
    status: 'SUCCEEDED',
    inputs: [],
    outputs: main,
    position: { x: 0, y: 96 },
  },
  check: {
    id: 'check-stock',
    kind: 'action',
    name: 'Check stock',
    summary: 'Looks up every product of the order in the warehouse list.',
    connector: { src: placeholderLogo, name: 'Warehouse' },
    status: 'SUCCEEDED',
    inputs: main,
    outputs: main,
    position: { x: 320, y: 96 },
  },
  branch: {
    id: 'in-stock',
    kind: 'control',
    controlType: 'if',
    name: 'In stock?',
    status: 'SUCCEEDED',
    inputs: main,
    outputs: [
      { id: 'true', label: 'true' },
      { id: 'false', label: 'false' },
    ],
    position: { x: 640, y: 128 },
  },
  charge: {
    id: 'charge-card',
    kind: 'action',
    name: 'Charge card',
    summary: 'Charges the customer card for the total of the order.',
    connector: { src: placeholderLogo, name: 'Payments' },
    status: 'SUCCEEDED_WITH_ERRORS',
    uncertainCount: 2,
    inputs: main,
    outputs: main,
    position: { x: 880, y: 16 },
  },
  backorder: {
    id: 'backorder',
    kind: 'action',
    name: 'Create back order',
    summary: 'Saves the missing products to ship when they arrive.',
    connector: { src: placeholderLogo, name: 'Warehouse' },
    status: 'RUNNING',
    inputs: main,
    outputs: main,
    position: { x: 880, y: 232 },
  },
  receipt: {
    id: 'send-receipt',
    kind: 'action',
    name: 'Send receipt',
    summary: 'Emails the receipt to the customer.',
    connector: { src: placeholderLogo, name: 'Email' },
    status: 'NOT_RUN',
    inputs: main,
    outputs: main,
    position: { x: 1200, y: 16 },
  },
} satisfies Record<string, GraphNode>;

export const exampleGraph: Graph = {
  nodes: Object.values(exampleNodes),
  edges: [
    { id: 'e1', source: 'order-received', sourceOutput: 'main', target: 'check-stock', targetInput: 'main', items: 12, active: true },
    { id: 'e2', source: 'check-stock', sourceOutput: 'main', target: 'in-stock', targetInput: 'main', items: 12, active: true },
    { id: 'e3', source: 'in-stock', sourceOutput: 'true', target: 'charge-card', targetInput: 'main', items: 9, active: true },
    { id: 'e4', source: 'in-stock', sourceOutput: 'false', target: 'backorder', targetInput: 'main', items: 3 },
    { id: 'e5', source: 'charge-card', sourceOutput: 'main', target: 'send-receipt', targetInput: 'main', items: 7, active: true },
    { id: 'e6', source: 'backorder', sourceOutput: 'main', target: 'check-stock', targetInput: 'main', items: 3, loopBack: true },
  ],
};

/** The same flow without an execution: no states, no counts. */
export const exampleDraft: Graph = {
  nodes: exampleGraph.nodes.map((node) => ({ ...node, status: undefined, uncertainCount: undefined })),
  edges: exampleGraph.edges.map((edge) => ({ ...edge, items: undefined, active: undefined })),
};

export const exampleCatalog: CatalogItem[] = [
  {
    id: 'schedule',
    kind: 'trigger',
    name: 'Schedule',
    summary: 'Starts the flow at set times.',
    group: 'Triggers',
    connector: { src: placeholderLogo, name: 'Clock' },
    inputs: [],
    outputs: main,
  },
  {
    id: 'webhook',
    kind: 'trigger',
    name: 'Webhook',
    summary: 'Starts the flow when another system calls a web address.',
    group: 'Triggers',
    connector: { src: placeholderLogo, name: 'Webhook' },
    keywords: ['http', 'request'],
    inputs: [],
    outputs: main,
  },
  {
    id: 'http-request',
    kind: 'action',
    name: 'HTTP request',
    summary: 'Calls a web address and returns the answer.',
    group: 'Actions',
    connector: { src: placeholderLogo, name: 'HTTP' },
    inputs: main,
    outputs: main,
  },
  {
    id: 'send-email',
    kind: 'action',
    name: 'Send email',
    summary: 'Sends an email to one or more people.',
    group: 'Actions',
    connector: { src: placeholderLogo, name: 'Email' },
    inputs: main,
    outputs: main,
  },
  {
    id: 'if',
    kind: 'control',
    controlType: 'if',
    name: 'If',
    group: 'Control',
    inputs: main,
    outputs: [
      { id: 'true', label: 'true' },
      { id: 'false', label: 'false' },
    ],
  },
  {
    id: 'merge',
    kind: 'control',
    controlType: 'merge',
    name: 'Merge',
    group: 'Control',
    inputs: [
      { id: 'a', label: 'a' },
      { id: 'b', label: 'b' },
    ],
    outputs: main,
  },
  { id: 'wait', kind: 'control', controlType: 'wait', name: 'Wait', group: 'Control', inputs: main, outputs: main },
];
