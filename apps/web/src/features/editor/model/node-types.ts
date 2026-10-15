// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { FlowNode } from './flow-document';

/**
 * The node types the editor can insert and draw before the node-type view of the API (SPEC-005
 * A10, cut ED4a) exists: the built-in types the flow format itself defines or names
 * (docs/reference/orcon-flow-1.md). When the API view arrives it replaces this table; only the
 * code that builds the palette from it changes.
 */

export type CanvasKind = 'action' | 'logic' | 'trigger' | 'agent' | 'placeholder';
export type CanvasLogic = 'if' | 'switch' | 'filter' | 'merge' | 'loop' | 'wait';
export type CanvasIconName =
  | 'webhook'
  | 'globe'
  | 'mail'
  | 'message'
  | 'database'
  | 'clock'
  | 'file'
  | 'code'
  | 'play'
  | 'bot'
  | 'zap'
  | 'help';

export type NodeGroup = 'triggers' | 'flow' | 'data';

export interface PortDef {
  readonly id: string;
  readonly label?: string;
}

export interface NodeTypeDef {
  /** Short key; a node added from the palette gets the id `add-<slug>-<n>`. */
  readonly slug: string;
  /** `<namespace>.<name>@<major>`. */
  readonly type: string;
  /** The BPMN element class of the node (orcon-flow-1.md). */
  readonly element: string;
  readonly kind: CanvasKind;
  readonly logic?: CanvasLogic;
  readonly icon?: CanvasIconName;
  readonly outputs: readonly PortDef[];
  readonly group: NodeGroup;
  /** Names the same node has elsewhere, so a search for them finds this type. */
  readonly aliases: readonly string[];
  /** The `parameters` and behaviour blocks a new node of this type starts with. */
  readonly initial: () => Pick<FlowNode, 'parameters'> & Partial<Pick<FlowNode, 'routing' | 'join' | 'loop'>>;
}

const MAIN: readonly PortDef[] = [{ id: 'main' }];

export const START_TYPE = 'orcon.start@1';
export const LOOP_TYPE = 'orcon.loop@1';
export const UNSUPPORTED_TYPE = 'orcon.unsupported@1';
/** Node ids the palette gives to new nodes start with this. */
export const ADDED_ID_PREFIX = 'add-';

export const NODE_TYPES: readonly NodeTypeDef[] = [
  {
    slug: 'webhook',
    type: START_TYPE,
    element: 'startEvent',
    kind: 'trigger',
    icon: 'webhook',
    outputs: MAIN,
    group: 'triggers',
    aliases: ['Webhook', 'HTTP trigger', 'On webhook call'],
    initial: () => ({ parameters: { method: 'POST', path: '/webhook' } }),
  },
  {
    slug: 'manual',
    type: START_TYPE,
    element: 'startEvent',
    kind: 'trigger',
    icon: 'play',
    outputs: MAIN,
    group: 'triggers',
    aliases: ['Manual trigger', 'When clicking Execute'],
    initial: () => ({ parameters: {} }),
  },
  {
    slug: 'http',
    type: 'orcon.http@1',
    element: 'serviceTask',
    kind: 'action',
    icon: 'globe',
    outputs: MAIN,
    group: 'data',
    aliases: ['HTTP Request', 'REST', 'API call', 'fetch'],
    initial: () => ({ parameters: { method: 'GET', url: '' } }),
  },
  {
    slug: 'whatsapp',
    type: 'orcon.whatsapp.whatsapp@1',
    element: 'serviceTask',
    kind: 'action',
    icon: 'message',
    outputs: MAIN,
    group: 'data',
    aliases: ['WhatsApp', 'WhatsApp Cloud API', 'Send WhatsApp message', 'Message'],
    initial: () => ({ parameters: { operation: 'message.send' } }),
  },
  {
    slug: 'llm',
    type: 'orcon.ai.llm@1',
    element: 'serviceTask',
    kind: 'agent',
    icon: 'bot',
    outputs: MAIN,
    group: 'data',
    aliases: ['Language model', 'LLM', 'Chat model', 'Ask a model'],
    initial: () => ({ parameters: { model: { provider: '', name: '' }, inputField: 'text', responseFormat: 'TEXT' } }),
  },
  {
    slug: 'agent',
    type: 'orcon.ai.agent@1',
    element: 'serviceTask',
    kind: 'agent',
    icon: 'bot',
    outputs: MAIN,
    group: 'data',
    aliases: ['AI agent', 'Agent', 'Tool calling'],
    initial: () => ({ parameters: { model: { provider: '', name: '' }, inputField: 'text' } }),
  },
  {
    slug: 'set',
    type: 'orcon.set@1',
    element: 'task',
    kind: 'action',
    icon: 'file',
    outputs: MAIN,
    group: 'data',
    aliases: ['Set', 'Edit Fields', 'Assign', 'Transform'],
    initial: () => ({ parameters: {} }),
  },
  {
    slug: 'noop',
    type: 'orcon.noop@1',
    element: 'task',
    kind: 'action',
    icon: 'zap',
    outputs: MAIN,
    group: 'data',
    aliases: ['No Operation', 'NoOp', 'Do nothing', 'Placeholder'],
    initial: () => ({ parameters: {} }),
  },
  {
    slug: 'if',
    type: 'orcon.if@1',
    element: 'exclusiveGateway',
    kind: 'logic',
    logic: 'if',
    outputs: [
      { id: 'true', label: 'true' },
      { id: 'false', label: 'false' },
    ],
    group: 'flow',
    aliases: ['IF', 'Condition', 'Branch', 'Filter'],
    initial: () => ({ parameters: { condition: '' }, routing: 'EXCLUSIVE' }),
  },
  {
    slug: 'merge',
    type: 'orcon.merge@1',
    element: 'task',
    kind: 'logic',
    logic: 'merge',
    outputs: MAIN,
    group: 'flow',
    aliases: ['Merge', 'Join', 'Combine'],
    initial: () => ({ parameters: {}, join: { mode: 'ALL', data: 'APPEND' } }),
  },
  {
    slug: 'loop',
    type: LOOP_TYPE,
    element: 'task',
    kind: 'logic',
    logic: 'loop',
    outputs: [{ id: 'loop' }, { id: 'done' }],
    group: 'flow',
    aliases: ['Loop', 'Loop Over Items', 'Split in Batches', 'Repeat'],
    initial: () => ({ parameters: {}, loop: { maxIterations: 10 } }),
  },
  {
    slug: 'call',
    type: 'orcon.callFlow@1',
    element: 'callActivity',
    kind: 'action',
    icon: 'play',
    outputs: MAIN,
    group: 'flow',
    aliases: ['Sub-flow', 'Call flow', 'Execute Workflow', 'Run flow'],
    // The `call` block is written when a flow is chosen; until then the node has none (publishing asks for it, orcon-flow-1.md V11).
    initial: () => ({ parameters: {} }),
  },
  {
    slug: 'end',
    type: 'orcon.end@1',
    element: 'endEvent',
    kind: 'action',
    icon: 'zap',
    outputs: [],
    group: 'flow',
    aliases: ['End', 'Stop', 'Finish'],
    initial: () => ({ parameters: {} }),
  },
];

const bySlug = new Map(NODE_TYPES.map((def) => [def.slug, def]));

export function typeDefBySlug(slug: string): NodeTypeDef | undefined {
  return bySlug.get(slug);
}

/** The definition of a node in a document. A start event is a webhook when it has a path, a manual trigger otherwise. */
export function typeDefOf(node: Pick<FlowNode, 'type' | 'parameters'>): NodeTypeDef | undefined {
  if (node.type === START_TYPE) return bySlug.get(typeof node.parameters['path'] === 'string' ? 'webhook' : 'manual');
  return NODE_TYPES.find((def) => def.type === node.type);
}

/** Reads the palette slug out of an id the canvas made from a palette entry (`add-http-2` is `http`). */
export function slugOfAddedId(id: string): string | undefined {
  if (!id.startsWith(ADDED_ID_PREFIX)) return undefined;
  const match = /^add-([a-z0-9]+)-\d+$/.exec(id);
  return match?.[1];
}

const text = (value: unknown): string => (typeof value === 'string' ? value : '');

/** One short line under the node name, from what the node is set to (a data-derived line, never prose). */
export function subtitleOf(node: Pick<FlowNode, 'type' | 'parameters'>): string | undefined {
  const parameters = node.parameters;
  if (node.type === START_TYPE) {
    const path = text(parameters['path']);
    return path === '' ? undefined : `${text(parameters['method']) || 'GET'} ${path}`;
  }
  if (node.type === 'orcon.http@1') {
    const url = text(parameters['url']);
    const method = text(parameters['method']) || 'GET';
    if (url === '') return undefined;
    const target = url.startsWith('=') ? url.slice(1) : url;
    try {
      return `${method} ${new URL(target).hostname}`;
    } catch {
      return `${method} ${target}`.slice(0, 60);
    }
  }
  if (node.type === 'orcon.if@1') {
    const condition = text(parameters['condition']);
    const inner = /^=\{\{\s*(.*?)\s*\}\}$/.exec(condition)?.[1] ?? condition.replace(/^=/, '');
    return inner === '' ? undefined : inner.slice(0, 60);
  }
  return undefined;
}
