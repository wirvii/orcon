// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Example flows for stories, tests and the screen templates. No request is made: the logos
// are small images embedded in the file.

import type { CatalogItem, Graph, GraphEdge, GraphNode, StickyNoteData } from './model';

// Original placeholder logos (rounded squares with a simple glyph), as data URIs.
export const logos = {
  shop: 'data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCA0MCA0MCI+PHJlY3QgeD0iNCIgeT0iNCIgd2lkdGg9IjMyIiBoZWlnaHQ9IjMyIiByeD0iOSIgZmlsbD0iIzBFNzQ5MCIvPjxwYXRoIGQ9Ik0xMiAxNWgxNmwtMS41IDExaC0xM3oiIGZpbGw9IiNFQ0ZFRkYiLz48cGF0aCBkPSJNMTYgMTVhNCA0IDAgMCAxIDggMCIgZmlsbD0ibm9uZSIgc3Ryb2tlPSIjRUNGRUZGIiBzdHJva2Utd2lkdGg9IjIuNCIgc3Ryb2tlLWxpbmVjYXA9InJvdW5kIi8+PC9zdmc+',
  warehouse:
    'data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCA0MCA0MCI+PHJlY3QgeD0iNCIgeT0iNCIgd2lkdGg9IjMyIiBoZWlnaHQ9IjMyIiByeD0iOSIgZmlsbD0iIzMzNDE1NSIvPjxwYXRoIGQ9Ik0xMCAxN2wxMC02IDEwIDZ2MTJIMTB6IiBmaWxsPSIjRjFGNUY5Ii8+PHJlY3QgeD0iMTYiIHk9IjIyIiB3aWR0aD0iOCIgaGVpZ2h0PSI3IiBmaWxsPSIjMzM0MTU1Ii8+PC9zdmc+',
  payments:
    'data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCA0MCA0MCI+PHJlY3QgeD0iNCIgeT0iNCIgd2lkdGg9IjMyIiBoZWlnaHQ9IjMyIiByeD0iOSIgZmlsbD0iIzE1ODAzRCIvPjxyZWN0IHg9IjkiIHk9IjEyIiB3aWR0aD0iMjIiIGhlaWdodD0iMTYiIHJ4PSIzIiBmaWxsPSIjRjBGREY0Ii8+PHJlY3QgeD0iOSIgeT0iMTYiIHdpZHRoPSIyMiIgaGVpZ2h0PSIzLjUiIGZpbGw9IiMxNTgwM0QiLz48L3N2Zz4=',
  email:
    'data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCA0MCA0MCI+PHJlY3QgeD0iNCIgeT0iNCIgd2lkdGg9IjMyIiBoZWlnaHQ9IjMyIiByeD0iOSIgZmlsbD0iIzQ3NTU2OSIvPjxyZWN0IHg9IjkiIHk9IjEyIiB3aWR0aD0iMjIiIGhlaWdodD0iMTYiIHJ4PSIzIiBmaWxsPSIjRjhGQUZDIi8+PHBhdGggZD0iTTEwIDE0bDEwIDggMTAtOCIgZmlsbD0ibm9uZSIgc3Ryb2tlPSIjNDc1NTY5IiBzdHJva2Utd2lkdGg9IjIuMiIgc3Ryb2tlLWxpbmVjYXA9InJvdW5kIiBzdHJva2UtbGluZWpvaW49InJvdW5kIi8+PC9zdmc+',
  chat: 'data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCA0MCA0MCI+PHJlY3QgeD0iNCIgeT0iNCIgd2lkdGg9IjMyIiBoZWlnaHQ9IjMyIiByeD0iOSIgZmlsbD0iIzA4OTFCMiIvPjxwYXRoIGQ9Ik0xMSAxMmgxOHYxMkgxOWwtNSA1di01aC0zeiIgZmlsbD0iI0VDRkVGRiIvPjwvc3ZnPg==',
} as const;

/** Kept for the stories of the base components. */
export const placeholderLogo = logos.shop;

const main = [{ id: 'main' }];
const trueFalse = [
  { id: 'true', label: 'true' },
  { id: 'false', label: 'false' },
];

export const exampleNodes = {
  webhook: {
    id: 'webhook',
    kind: 'trigger',
    name: 'Webhook',
    subtitle: 'On POST /orders',
    icon: 'webhook',
    status: 'SUCCEEDED',
    inputs: [],
    outputs: main,
    position: { x: 40, y: 160 },
  },
  validate: {
    id: 'validate',
    kind: 'logic',
    logicType: 'if',
    name: 'Validate order',
    subtitle: 'If total > 0',
    status: 'SUCCEEDED',
    inputs: main,
    outputs: trueFalse,
    position: { x: 240, y: 160 },
  },
  http: {
    id: 'http',
    kind: 'action',
    name: 'HTTP Request',
    subtitle: 'GET inventory',
    icon: 'globe',
    status: 'RUNNING',
    inputs: main,
    outputs: main,
    position: { x: 440, y: 60 },
  },
  slack: {
    id: 'slack',
    kind: 'action',
    name: 'Slack',
    subtitle: 'Send message',
    connector: { src: logos.chat, name: 'Slack' },
    status: 'UNCERTAIN',
    inputs: main,
    outputs: main,
    position: { x: 640, y: 60 },
  },
  gmail: {
    id: 'gmail',
    kind: 'action',
    name: 'Gmail',
    subtitle: 'Send email',
    connector: { src: logos.email, name: 'Gmail' },
    status: 'FAILED',
    statusCode: '401',
    inputs: main,
    outputs: main,
    position: { x: 440, y: 280 },
  },
  merge: {
    id: 'merge',
    kind: 'logic',
    logicType: 'merge',
    name: 'Merge',
    subtitle: 'Append',
    disabled: true,
    inputs: [
      { id: 'a', label: 'a' },
      { id: 'b', label: 'b' },
    ],
    outputs: main,
    position: { x: 840, y: 160 },
  },
} satisfies Record<string, GraphNode>;

const exampleNote: StickyNoteData = {
  id: 'note-1',
  position: { x: 40, y: 340 },
  size: { w: 220, h: 120 },
  color: 1,
  body: 'Orders above 5,000 need a manual review first. Handled by the review flow.',
};

const exampleEdges: GraphEdge[] = [
  { id: 'e1', source: 'webhook', sourceOutput: 'main', target: 'validate', targetInput: 'main', items: 1, state: 'succeeded' },
  { id: 'e2', source: 'validate', sourceOutput: 'true', target: 'http', targetInput: 'main', items: 1, state: 'succeeded' },
  { id: 'e3', source: 'validate', sourceOutput: 'false', target: 'gmail', targetInput: 'main', items: 1, state: 'succeeded' },
  { id: 'e4', source: 'http', sourceOutput: 'main', target: 'slack', targetInput: 'main', items: 3, state: 'running' },
  { id: 'e5', source: 'slack', sourceOutput: 'main', target: 'merge', targetInput: 'a', state: 'uncertain' },
  { id: 'e6', source: 'gmail', sourceOutput: 'main', target: 'merge', targetInput: 'b', items: 0, state: 'failed' },
];

/**
 * The order-intake flow with one node in every execution state. The states are spread out to show
 * each of them at once; they are not one coherent run.
 */
export const exampleGraph: Graph = {
  nodes: Object.values(exampleNodes),
  edges: exampleEdges,
  notes: [exampleNote],
};

/** The same flow without an execution: no states, no counts, nothing disabled. */
export const exampleDraft: Graph = {
  nodes: exampleGraph.nodes.map((node) => ({ ...node, status: undefined, statusCode: undefined, uncertainCount: undefined, disabled: undefined })),
  edges: exampleEdges.map((edge) => ({ ...edge, items: undefined, state: undefined })),
  notes: [exampleNote],
};

/** A run that finished well: every step succeeded and the counts add up. */
export const exampleSucceeded: Graph = {
  nodes: exampleDraft.nodes.map((node) => ({ ...node, status: 'SUCCEEDED' })),
  edges: exampleEdges.map((edge) => ({ ...edge, items: edge.id === 'e6' ? 1 : 1, state: 'succeeded' })),
  notes: [exampleNote],
};

/** An agent node with its configuration blocks, next to the trigger that feeds it. */
export const agentGraph: Graph = {
  nodes: [
    { ...exampleNodes.webhook, id: 'chat-trigger', name: 'Chat message', subtitle: 'On new message', status: undefined, position: { x: 40, y: 80 } },
    {
      id: 'agent',
      kind: 'agent',
      name: 'Support agent',
      subtitle: 'Tools agent',
      icon: 'bot',
      inputs: main,
      outputs: main,
      ai: [
        { kind: 'model', set: true, detail: 'Language model' },
        { kind: 'memory', set: true, detail: 'Window memory' },
        { kind: 'tools', set: true, detail: '3 tools' },
        { kind: 'parser', set: false },
      ],
      position: { x: 240, y: 80 },
    },
  ],
  edges: [{ id: 'a1', source: 'chat-trigger', sourceOutput: 'main', target: 'agent', targetInput: 'main' }],
};

/** A node that was imported but has no equivalent in Orcon. */
export const placeholderNode: GraphNode = {
  id: 'unsupported',
  kind: 'placeholder',
  name: 'Custom code',
  subtitle: 'orcon.unsupported@1',
  disabled: true,
  inputs: main,
  outputs: main,
  position: { x: 0, y: 0 },
};

/** A flow of `count` nodes laid out in rows, for the performance story. */
export function largeGraph(count: number, rowLength = 25): Graph {
  const nodes: GraphNode[] = [];
  const edges: GraphEdge[] = [];
  for (let index = 0; index < count; index += 1) {
    const column = index % rowLength;
    const row = Math.floor(index / rowLength);
    nodes.push({
      id: `n${index}`,
      kind: index === 0 ? 'trigger' : 'action',
      name: `Step ${index + 1}`,
      subtitle: 'Do something',
      icon: index % 3 === 0 ? 'globe' : index % 3 === 1 ? 'database' : 'mail',
      inputs: index === 0 ? [] : main,
      outputs: main,
      position: { x: column * 200, y: row * 160 },
    });
    if (index > 0 && column > 0) {
      edges.push({ id: `l${index}`, source: `n${index - 1}`, sourceOutput: 'main', target: `n${index}`, targetInput: 'main' });
    }
  }
  return { nodes, edges };
}

export const exampleCatalog: CatalogItem[] = [
  { id: 'schedule', kind: 'trigger', name: 'Schedule', subtitle: 'Starts the flow at set times', group: 'Triggers', icon: 'clock', inputs: [], outputs: main },
  {
    id: 'webhook',
    kind: 'trigger',
    name: 'Webhook',
    subtitle: 'Starts the flow when another system calls a web address',
    group: 'Triggers',
    icon: 'webhook',
    keywords: ['http', 'request'],
    inputs: [],
    outputs: main,
  },
  { id: 'manual', kind: 'trigger', name: 'Manual', subtitle: 'Starts the flow when you press Execute', group: 'Triggers', icon: 'play', inputs: [], outputs: main },
  {
    id: 'http-request',
    kind: 'action',
    name: 'HTTP Request',
    subtitle: 'Calls a web address and returns the answer',
    group: 'Actions',
    icon: 'globe',
    inputs: main,
    outputs: main,
  },
  {
    id: 'send-email',
    kind: 'action',
    name: 'Send email',
    subtitle: 'Sends an email to one or more people',
    group: 'Actions',
    connector: { src: logos.email, name: 'Email' },
    inputs: main,
    outputs: main,
  },
  {
    id: 'slack-message',
    kind: 'action',
    name: 'Slack',
    subtitle: 'Sends a message to a channel',
    group: 'Actions',
    connector: { src: logos.chat, name: 'Slack' },
    inputs: main,
    outputs: main,
  },
  { id: 'database', kind: 'action', name: 'Database', subtitle: 'Reads or writes rows', group: 'Actions', icon: 'database', inputs: main, outputs: main },
  { id: 'if', kind: 'logic', logicType: 'if', name: 'If', subtitle: 'Splits the flow in two', group: 'Logic', inputs: main, outputs: trueFalse },
  {
    id: 'switch',
    kind: 'logic',
    logicType: 'switch',
    name: 'Switch',
    subtitle: 'Sends items down one of several paths',
    group: 'Logic',
    inputs: main,
    outputs: [
      { id: 'a', label: 'case a' },
      { id: 'b', label: 'case b' },
      { id: 'c', label: 'case c' },
    ],
  },
  { id: 'filter', kind: 'logic', logicType: 'filter', name: 'Filter', subtitle: 'Keeps only the items that match', group: 'Logic', inputs: main, outputs: main },
  {
    id: 'merge',
    kind: 'logic',
    logicType: 'merge',
    name: 'Merge',
    subtitle: 'Joins two paths into one',
    group: 'Logic',
    inputs: [
      { id: 'a', label: 'a' },
      { id: 'b', label: 'b' },
    ],
    outputs: main,
  },
  { id: 'loop', kind: 'logic', logicType: 'loop', name: 'Loop', subtitle: 'Repeats steps for each item', group: 'Logic', inputs: main, outputs: main },
  { id: 'wait', kind: 'logic', logicType: 'wait', name: 'Wait', subtitle: 'Pauses the flow', group: 'Logic', inputs: main, outputs: main },
  { id: 'agent', kind: 'agent', name: 'Agent', subtitle: 'An agent that uses tools', group: 'AI', icon: 'bot', inputs: main, outputs: main },
];
