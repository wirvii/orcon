// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { expect, waitFor } from 'storybook/test';

import { exampleNodes } from '../canvas/fixtures';
import type { Graph, GraphNode } from '../canvas/model';
import { orderItems } from '../editor/fixtures';
import type { EditorFrameProps } from '../screens/EditorFrame';

import type { ParityEnv } from './env';

export interface ParityScript {
  /** The editor the row starts from. */
  frame: Omit<EditorFrameProps, 'onGraphChange'>;
  /** The mouse script. Needs real input, so only the browser test runs it. */
  pointer?: (env: ParityEnv) => Promise<void>;
  /** The keyboard script. It also plays in Storybook, where it produces the end state the baselines show. */
  keyboard: (env: ParityEnv) => Promise<void>;
  /** Checks the end state. Both scripts must reach it. */
  verify: (env: ParityEnv) => Promise<void>;
}

// ---- Starting points ---------------------------------------------------------------------------

const place = (node: GraphNode, x: number, y: number): GraphNode => ({
  ...node,
  status: undefined,
  statusCode: undefined,
  uncertainCount: undefined,
  disabled: undefined,
  position: { x, y },
});

const webhook = place(exampleNodes.webhook, 40, 160);
const http = place(exampleNodes.http, 240, 160);
const slack = place(exampleNodes.slack, 440, 160);
const e1 = { id: 'e1', source: 'webhook', sourceOutput: 'main', target: 'http', targetInput: 'main' };
const e2 = { id: 'e2', source: 'http', sourceOutput: 'main', target: 'slack', targetInput: 'main' };

/** A trigger and a node: the output of the second one is free. */
const pair: Graph = { nodes: [webhook, http], edges: [e1] };
/** The same, with a third node already connected. */
const chain: Graph = { nodes: [webhook, http, slack], edges: [e1, e2] };
/** Three nodes with the last one unconnected. */
const loose: Graph = { nodes: [webhook, http, slack], edges: [e1] };

const fast = { runDelay: 90 } as const;

// ---- Small helpers -----------------------------------------------------------------------------

const selected = (env: ParityEnv, id: string) => env.nodeElement(id)?.classList.contains('selected') ?? false;
const nodeState = (env: ParityEnv, id: string) => env.nodeElement(id)?.querySelector('[data-canvas-node]')?.getAttribute('data-node-state');
const byName = (graph: Graph, name: string) => graph.nodes.find((node) => node.name === name);

/** Opens the palette from a plus button or the A key, types a search and picks the first match. */
async function pickFromPalette(env: ParityEnv, name: string) {
  await env.click(await env.option(new RegExp(name)));
}
async function pickWithKeyboard(env: ParityEnv, search: string) {
  const field = await env.q.findByRole('combobox', { name: 'Search nodes' });
  await waitFor(() => expect(field).toHaveFocus());
  await env.type(field, search);
  await env.keys('{Enter}');
}

/** After adding a node, it exists, is connected from `source` and is selected. */
async function expectAddedAfter(env: ParityEnv, name: string, sourceId: string) {
  await waitFor(() => expect(byName(env.getGraph(), name)).toBeDefined());
  const graph = env.getGraph();
  const added = byName(graph, name) as GraphNode;
  const source = graph.nodes.find((node) => node.id === sourceId) as GraphNode;
  expect(added.position).toEqual({ x: source.position.x + 200, y: source.position.y });
  expect(graph.edges.some((edge) => edge.source === sourceId && edge.target === added.id)).toBe(true);
  await waitFor(() => expect(selected(env, added.id)).toBe(true));
}

async function expectCompleted(env: ParityEnv, id: string) {
  await waitFor(() => expect(nodeState(env, id)).toBe('succeeded'), { timeout: 4000 });
}

export const parityScripts: Record<string, ParityScript> = {
  // P-01: the first step of an empty canvas.
  'P-01': {
    frame: { graph: { nodes: [], edges: [] }, ...fast },
    pointer: async (env) => {
      await env.unit('first step', 2, async () => {
        await env.click(await env.button('Add first step'));
        await env.click(await env.option(/Schedule/));
      });
    },
    keyboard: async (env) => {
      await env.reach(async () => env.focus(await env.button('Add first step')));
      await env.unit('first step', 2, async () => {
        await env.keys('{Enter}');
        await env.keys('{Enter}');
      });
    },
    verify: async (env) => {
      await waitFor(() => expect(env.getGraph().nodes).toHaveLength(1));
      const added = env.getGraph().nodes[0] as GraphNode;
      expect(added.kind).toBe('trigger');
      expect(added.name).toBe('Schedule');
      await waitFor(() => expect(selected(env, added.id)).toBe(true));
    },
  },

  // P-02: add a node after a node, from its plus button or the A key.
  'P-02': {
    frame: { graph: pair, ...fast },
    pointer: async (env) => {
      const target = await env.node('HTTP Request');
      await env.hover(target);
      const plus = target.querySelector<HTMLElement>('[data-port="output"]') as HTMLElement;
      await env.unit('add after', 2, async () => {
        await env.click(plus);
        await pickFromPalette(env, 'Send email');
      });
    },
    keyboard: async (env) => {
      await env.reach(async () => env.focus(await env.node('HTTP Request')));
      await env.unit('add after', 2, async () => {
        await env.keys('a');
        await pickWithKeyboard(env, 'email');
      });
    },
    verify: async (env) => expectAddedAfter(env, 'Send email', 'http'),
  },

  // P-03: add after an output that has no connection yet, from the dashed stub.
  'P-03': {
    frame: { graph: pair, ...fast },
    pointer: async (env) => {
      const target = await env.node('HTTP Request');
      await env.hover(target);
      const stub = target.querySelector<HTMLElement>('[data-output-stub] button') as HTMLElement;
      await env.unit('add from the stub', 2, async () => {
        await env.click(stub);
        await pickFromPalette(env, 'Send email');
      });
    },
    keyboard: async (env) => {
      const target = await env.node('HTTP Request');
      await env.reach(async () => {
        env.focus(target);
        env.focus(target.querySelector<HTMLElement>('[data-output-stub] button') as HTMLElement);
      });
      await env.unit('add from the stub', 2, async () => {
        await env.keys('{Enter}');
        await pickWithKeyboard(env, 'email');
      });
    },
    verify: async (env) => expectAddedAfter(env, 'Send email', 'http'),
  },

  // P-04: insert a node on a connection.
  'P-04': {
    frame: { graph: pair, ...fast },
    pointer: async (env) => {
      await env.hover(await env.edgeLine('e1'));
      const add = await env.button('Add node here');
      await env.unit('insert on the connection', 2, async () => {
        await env.click(add);
        await pickFromPalette(env, 'Send email');
      });
    },
    keyboard: async (env) => {
      await env.reach(async () => env.focus(await env.edge('e1')));
      await env.unit('insert on the connection', 2, async () => {
        await env.keys('{Enter}');
        await pickWithKeyboard(env, 'email');
      });
    },
    verify: async (env) => {
      await waitFor(() => expect(byName(env.getGraph(), 'Send email')).toBeDefined());
      const graph = env.getGraph();
      const added = byName(graph, 'Send email') as GraphNode;
      // The connection is split and reconnected through the new node, which takes the target's column.
      expect(graph.edges.some((edge) => edge.id === 'e1')).toBe(false);
      expect(graph.edges.some((edge) => edge.source === 'webhook' && edge.target === added.id)).toBe(true);
      expect(graph.edges.some((edge) => edge.source === added.id && edge.target === 'http')).toBe(true);
      expect(added.position.x).toBe(240);
      expect(graph.nodes.find((node) => node.id === 'http')?.position.x).toBe(440);
    },
  },

  // P-05: delete a connection.
  'P-05': {
    frame: { graph: pair, ...fast },
    pointer: async (env) => {
      await env.hover(await env.edgeLine('e1'));
      const remove = await env.button('Delete connection');
      await env.unit('delete the connection', 1, async () => {
        await env.click(remove);
      });
    },
    keyboard: async (env) => {
      await env.reach(async () => env.focus(await env.edge('e1')));
      await env.unit('delete the connection', 1, async () => {
        await env.keys('{Delete}');
      });
    },
    verify: async (env) => {
      await waitFor(() => expect(env.getGraph().edges).toHaveLength(0));
    },
  },

  // P-06: connect two nodes.
  'P-06': {
    frame: { graph: loose, ...fast },
    pointer: async (env) => {
      const from = (await env.node('HTTP Request')).querySelector<HTMLElement>('[data-port="output"]') as HTMLElement;
      const to = (await env.node('Slack')).querySelector<HTMLElement>('[data-port="input"]') as HTMLElement;
      await env.unit('connect', 1, async () => {
        await env.drag(from, to);
      });
    },
    keyboard: async (env) => {
      // Selecting the two nodes is reaching them; C then connects the one on the left to the other.
      await env.reach(async () => {
        const left = await env.node('HTTP Request');
        env.focus(left);
        await env.driver.keyboard(' ');
        const right = await env.node('Slack');
        env.focus(right);
        await env.driver.keyboard(' ');
      });
      await env.unit('connect', 1, async () => {
        await env.keys('c');
      });
    },
    verify: async (env) => {
      await waitFor(() => expect(env.getGraph().edges.some((edge) => edge.source === 'http' && edge.target === 'slack')).toBe(true));
    },
  },

  // P-07: move, multi-select and marquee.
  'P-07': {
    frame: { graph: chain, ...fast },
    pointer: async (env) => {
      const target = await env.node('HTTP Request');
      const before = env.getGraph().nodes.find((node) => node.id === 'http')?.position;
      await env.unit('move', 1, async () => {
        await env.drag(target, env.pane(), { targetPosition: { x: 260, y: 120 } });
      });
      await waitFor(() => expect(env.getGraph().nodes.find((node) => node.id === 'http')?.position).not.toEqual(before));
      const moved = env.getGraph().nodes.find((node) => node.id === 'http')?.position as { x: number; y: number };
      expect(Math.abs(moved.x % 20)).toBe(0);
      expect(Math.abs(moved.y % 20)).toBe(0);
      await env.unit('select several', 1, async () => {
        await env.click(await env.node('Webhook'));
      });
      await env.unit('add to the selection', 1, async () => {
        await env.shiftClick(await env.node('Slack'));
      });
    },
    keyboard: async (env) => {
      const target = await env.node('HTTP Request');
      const before = env.getGraph().nodes.find((node) => node.id === 'http')?.position as { x: number; y: number };
      await env.reach(() => env.focus(target));
      await env.unit('move', 1, async () => {
        await env.keys('{Shift>}{ArrowRight}{/Shift}');
      });
      await waitFor(() => expect(env.getGraph().nodes.find((node) => node.id === 'http')?.position).toEqual({ x: before.x + 20, y: before.y }));
      await env.unit('select several', 1, async () => {
        await env.keys('{Control>}a{/Control}');
      });
    },
    verify: async (env) => {
      await waitFor(() => expect(selected(env, 'webhook') || selected(env, 'slack') || selected(env, 'http')).toBe(true));
      // Every node of the flow sits on the 20 px grid.
      for (const node of env.getGraph().nodes) {
        expect(Math.abs(node.position.x % 20)).toBe(0);
        expect(Math.abs(node.position.y % 20)).toBe(0);
      }
    },
  },

  // P-08: the quick actions of a node, here the menu with more actions.
  'P-08': {
    frame: { graph: chain, ...fast },
    pointer: async (env) => {
      await env.hover(await env.node('HTTP Request'));
      const more = await env.button('More actions');
      await env.unit('more actions', 1, async () => {
        await env.click(more);
      });
    },
    keyboard: async (env) => {
      await env.reach(async () => env.focus(await env.node('HTTP Request')));
      const more = await env.button('More actions');
      await env.reach(() => env.focus(more));
      await env.unit('more actions', 1, async () => {
        await env.keys('{Enter}');
      });
    },
    verify: async (env) => {
      for (const name of ['Rename', 'Duplicate', 'Pin data', 'Copy', 'Open details']) await env.q.findByRole('menuitem', { name });
      const toolbar = await env.q.findByRole('toolbar', { name: 'Actions for HTTP Request' });
      for (const name of ['Run step', 'Disable node', 'Delete node']) expect(toolbar.querySelector(`[aria-label="${name}"]`)).not.toBeNull();
    },
  },

  // P-09: select a node and open it.
  'P-09': {
    frame: { graph: chain, ...fast },
    pointer: async (env) => {
      const target = await env.node('HTTP Request');
      await env.unit('select', 1, async () => {
        await env.click(target);
      });
      await waitFor(() => expect(selected(env, 'http')).toBe(true));
      await env.unit('open', 1, async () => {
        await env.dblClick(target);
      });
    },
    keyboard: async (env) => {
      await env.reach(async () => env.focus(await env.node('HTTP Request')));
      await env.unit('select', 1, async () => {
        await env.keys(' ');
      });
      await waitFor(() => expect(selected(env, 'http')).toBe(true));
      await env.unit('open', 1, async () => {
        await env.keys('{Enter}');
      });
    },
    verify: async (env) => {
      await env.q.findByRole('dialog', { name: 'HTTP Request details' });
    },
  },

  // P-10: the details view; Back or Escape closes it and returns focus to the node.
  'P-10': {
    frame: { graph: chain, defaultDetailsId: 'http', ...fast },
    pointer: async (env) => {
      const dialog = await env.q.findByRole('dialog', { name: 'HTTP Request details' });
      for (const pane of ['input', 'parameters', 'output']) expect(dialog.querySelector(`[data-pane="${pane}"]`)).not.toBeNull();
      await env.unit('back to the canvas', 1, async () => {
        await env.click(await env.button('Back to canvas'));
      });
    },
    keyboard: async (env) => {
      await env.q.findByRole('dialog', { name: 'HTTP Request details' });
      await env.unit('back to the canvas', 1, async () => {
        await env.keys('{Escape}');
      });
    },
    verify: async (env) => {
      await waitFor(() => expect(env.q.queryByRole('dialog', { name: 'HTTP Request details' })).toBeNull());
      await waitFor(() => expect(document.activeElement).toBe(env.nodeElement('http')));
    },
  },

  // P-11: switch the data view of a pane.
  'P-11': {
    frame: { graph: chain, defaultDetailsId: 'http', outputs: { webhook: { main: orderItems }, http: { main: orderItems } }, ...fast },
    pointer: async (env) => {
      const pane = (await env.q.findByRole('dialog', { name: 'HTTP Request details' })).querySelector('[data-data-pane="output"]') as HTMLElement;
      await env.unit('show as JSON', 1, async () => {
        await env.click(Array.from(pane.querySelectorAll('button')).find((button) => button.textContent === 'JSON') as HTMLElement);
      });
    },
    keyboard: async (env) => {
      const pane = (await env.q.findByRole('dialog', { name: 'HTTP Request details' })).querySelector('[data-data-pane="output"]') as HTMLElement;
      const json = Array.from(pane.querySelectorAll('button')).find((button) => button.textContent === 'JSON') as HTMLElement;
      await env.reach(() => env.focus(json));
      await env.unit('show as JSON', 1, async () => {
        await env.keys('{Enter}');
      });
    },
    verify: async (env) => {
      const pane = (await env.q.findByRole('dialog', { name: 'HTTP Request details' })).querySelector('[data-data-pane="output"]') as HTMLElement;
      await waitFor(() => expect(pane.querySelector('[role="tree"]')).not.toBeNull());
      expect(pane.querySelector('table')).toBeNull();
    },
  },

  // P-13: execute one step; it runs, then shows its result.
  'P-13': {
    frame: { graph: chain, ...fast },
    pointer: async (env) => {
      await env.hover(await env.node('HTTP Request'));
      const run = await env.button('Run step');
      await env.unit('run the step', 1, async () => {
        await env.click(run);
      });
    },
    keyboard: async (env) => {
      await env.reach(async () => env.focus(await env.node('HTTP Request')));
      await env.unit('run the step', 1, async () => {
        await env.keys('r');
      });
    },
    verify: async (env) => {
      await expectCompleted(env, 'http');
      // The connections around it show how many items went through.
      await waitFor(() => expect(env.root.querySelectorAll('[data-edge-label="succeeded"]').length).toBeGreaterThanOrEqual(2));
      expect(env.root.textContent).toContain('3 items');
    },
  },

  // P-14: execute the workflow; every step runs and the counts show on the connections.
  'P-14': {
    frame: { graph: chain, ...fast },
    pointer: async (env) => {
      await env.unit('execute the workflow', 1, async () => {
        await env.click(await env.button('Execute workflow'));
      });
    },
    keyboard: async (env) => {
      await env.reach(async () => env.focus(await env.button('Execute workflow')));
      await env.unit('execute the workflow', 1, async () => {
        await env.keys('{Enter}');
      });
    },
    verify: async (env) => {
      for (const id of ['webhook', 'http', 'slack']) await expectCompleted(env, id);
      await waitFor(() => expect(env.root.querySelectorAll('[data-edge-label="succeeded"]')).toHaveLength(2));
      await waitFor(() => expect(env.root.querySelector('[data-last-run="SUCCEEDED"]')).not.toBeNull());
    },
  },

  // P-15: pin the output data.
  'P-15': {
    frame: { graph: chain, defaultDetailsId: 'http', outputs: { webhook: { main: orderItems }, http: { main: orderItems } }, ...fast },
    pointer: async (env) => {
      const pane = (await env.q.findByRole('dialog', { name: 'HTTP Request details' })).querySelector('[data-data-pane="output"]') as HTMLElement;
      await env.unit('pin the output', 1, async () => {
        await env.click(Array.from(pane.querySelectorAll('button')).find((button) => button.textContent === 'Pin') as HTMLElement);
      });
    },
    keyboard: async (env) => {
      const pane = (await env.q.findByRole('dialog', { name: 'HTTP Request details' })).querySelector('[data-data-pane="output"]') as HTMLElement;
      const pin = Array.from(pane.querySelectorAll('button')).find((button) => button.textContent === 'Pin') as HTMLElement;
      await env.reach(() => env.focus(pin));
      await env.unit('pin the output', 1, async () => {
        await env.keys('{Enter}');
      });
    },
    verify: async (env) => {
      // The pane says so, the node has the pin badge and the Pinned chip.
      await env.q.findByText('This data is pinned');
      await waitFor(() => expect(env.root.querySelector('[data-id="http"] [data-pinned-badge]')).not.toBeNull());
      expect(env.root.querySelector('[data-id="http"] [data-node-chip="pinned"]')).not.toBeNull();
    },
  },

  // P-16: disable a node.
  'P-16': {
    frame: { graph: chain, ...fast },
    pointer: async (env) => {
      await env.hover(await env.node('HTTP Request'));
      const disable = await env.button('Disable node');
      await env.unit('disable', 1, async () => {
        await env.click(disable);
      });
    },
    keyboard: async (env) => {
      await env.reach(async () => env.focus(await env.node('HTTP Request')));
      await env.unit('disable', 1, async () => {
        await env.keys('d');
      });
    },
    verify: async (env) => {
      await waitFor(() => expect(nodeState(env, 'http')).toBe('disabled'));
      const tile = env.root.querySelector('[data-id="http"] [data-tile-surface]') as HTMLElement;
      expect(getComputedStyle(tile).borderTopStyle).toBe('dashed');
      expect(env.root.querySelector('[data-id="http"] [data-node-chip="disabled"]')).not.toBeNull();
    },
  },

  // P-17: a failed node reads at a glance, with nothing to do.
  'P-17': {
    frame: { graph: chain, overlay: { nodes: { slack: { status: 'FAILED', statusCode: '401' } } }, ...fast },
    pointer: async (env) => {
      await env.unit('read the node', 0, async () => undefined);
    },
    keyboard: async (env) => {
      await env.reach(async () => env.focus(await env.node('Slack')));
      await env.unit('read the node', 0, async () => undefined);
    },
    verify: async (env) => {
      const slackNode = await env.node('Slack');
      expect(slackNode).toHaveAccessibleName(/Status: Failed/);
      await waitFor(() => expect(nodeState(env, 'slack')).toBe('failed'));
      expect(slackNode.querySelector('[data-node-badge="FAILED"]')).not.toBeNull();
      expect(slackNode.querySelector('[data-node-chips]')).toHaveTextContent('Failed · 401');
    },
  },

  // P-18: an uncertain node reads at a glance and is never drawn as a failure.
  'P-18': {
    frame: { graph: chain, overlay: { nodes: { slack: { status: 'UNCERTAIN' } }, edges: { e2: { state: 'uncertain' } } }, ...fast },
    pointer: async (env) => {
      await env.unit('read the node', 0, async () => undefined);
    },
    keyboard: async (env) => {
      await env.reach(async () => env.focus(await env.node('Slack')));
      await env.unit('read the node', 0, async () => undefined);
    },
    verify: async (env) => {
      const slackNode = await env.node('Slack');
      expect(slackNode).toHaveAccessibleName(/Status: Uncertain/);
      await waitFor(() => expect(nodeState(env, 'slack')).toBe('uncertain'));
      expect(slackNode.querySelector('[data-node-chips]')).toHaveTextContent('Uncertain');
      expect(slackNode.querySelector('[data-node-badge="UNCERTAIN"]')).not.toBeNull();
      // It is never drawn as a failure: the tile, the badge and the chip carry no danger color.
      for (const part of ['[data-tile-surface]', '[data-node-badge]', '[data-node-chips]']) expect(slackNode.querySelector(part)?.outerHTML).not.toMatch(/status-danger/);
      expect(env.root.querySelector('[data-edge-state="uncertain"]')).not.toBeNull();
    },
  },

  // P-19: sticky notes: add, edit, resize and recolor.
  'P-19': {
    frame: { graph: { ...pair, notes: [] }, ...fast },
    pointer: async (env) => {
      await env.unit('add a note', 1, async () => {
        await env.click(await env.button('Add a sticky note'));
      });
      await waitFor(() => expect(env.getGraph().notes).toHaveLength(1));
      const area = await env.q.findByRole('textbox', { name: 'Note text' });
      await env.type(area, 'Check the totals');
      await env.driver.keyboard('{Control>}{Enter}{/Control}');
      const note = env.root.querySelector<HTMLElement>('[data-sticky-note]') as HTMLElement;
      await env.unit('recolor', 1, async () => {
        await env.click(await env.button('Color 3'));
      });
      const grip = env.root.querySelector<HTMLElement>('.react-flow__resize-control') as HTMLElement;
      const width = env.getGraph().notes?.[0]?.size.w ?? 0;
      await env.unit('resize', 1, async () => {
        await env.drag(grip, env.pane(), { targetPosition: { x: 700, y: 400 } });
      });
      await waitFor(() => expect(env.getGraph().notes?.[0]?.size.w).not.toBe(width));
      expect(note).toBeTruthy();
    },
    keyboard: async (env) => {
      await env.reach(async () => env.focus(await env.node('Webhook')));
      await env.unit('add a note', 1, async () => {
        await env.keys('{Shift>}s{/Shift}');
      });
      await waitFor(() => expect(env.getGraph().notes).toHaveLength(1));
      const area = await env.q.findByRole('textbox', { name: 'Note text' });
      await env.type(area, 'Check the totals');
      await env.driver.keyboard('{Control>}{Enter}{/Control}');
      const noteElement = await waitFor(() => {
        const element = env.root.querySelector<HTMLElement>('.react-flow__node-sticky');
        expect(element).not.toBeNull();
        return element as HTMLElement;
      });
      await env.reach(() => env.focus(noteElement));
      const width = env.getGraph().notes?.[0]?.size.w ?? 0;
      await env.unit('resize', 1, async () => {
        await env.keys('{Control>}{Shift>}{ArrowRight}{/Shift}{/Control}');
      });
      await waitFor(() => expect(env.getGraph().notes?.[0]?.size.w).toBe(width + 20));
      await env.reach(async () => env.focus(await env.button('Color 3')));
      await env.unit('recolor', 1, async () => {
        await env.keys('{Enter}');
      });
    },
    verify: async (env) => {
      await waitFor(() => expect(env.getGraph().notes?.[0]).toMatchObject({ body: 'Check the totals', color: 3 }));
    },
  },

  // P-20: zoom.
  'P-20': {
    frame: { graph: pair, ...fast },
    pointer: async (env) => {
      const before = env.zoom();
      await env.unit('zoom in', 1, async () => {
        await env.click(await env.button('Zoom in'));
      });
      await waitFor(() => expect(env.zoom()).toBeGreaterThan(before));
    },
    keyboard: async (env) => {
      await env.reach(async () => env.focus(await env.node('HTTP Request')));
      const before = env.zoom();
      await env.unit('zoom in', 1, async () => {
        await env.keys('+');
      });
      await waitFor(() => expect(env.zoom()).toBeGreaterThan(before));
    },
    verify: async (env) => {
      // Reset to 100 % shows only when the zoom is not 100 %.
      await env.button('Reset zoom to 100%');
    },
  },

  // P-21: the minimap appears while the canvas moves.
  'P-21': {
    frame: { graph: chain, ...fast },
    pointer: async (env) => {
      await env.unit('pan', 1, async () => {
        await env.wheel(env.pane(), { y: 80 });
      });
    },
    keyboard: async (env) => {
      await env.reach(async () => env.focus(await env.node('HTTP Request')));
      await env.unit('zoom', 1, async () => {
        await env.keys('+');
      });
    },
    verify: async (env) => {
      const minimap = await waitFor(() => {
        const element = env.root.querySelector<Element>('.react-flow__minimap');
        expect(element).not.toBeNull();
        return element as Element;
      });
      await waitFor(() => expect(minimap.getAttribute('class')).toContain('opacity-100'));
    },
  },

  // P-22: pan.
  'P-22': {
    frame: { graph: chain, ...fast },
    pointer: async (env) => {
      const before = getComputedStyle(env.root.querySelector('.react-flow__viewport') as HTMLElement).transform;
      await env.unit('pan', 1, async () => {
        await env.wheel(env.pane(), { x: 120, y: 60 });
      });
      await waitFor(() => expect(getComputedStyle(env.root.querySelector('.react-flow__viewport') as HTMLElement).transform).not.toBe(before));
    },
    keyboard: async (env) => {
      const before = getComputedStyle(env.root.querySelector('.react-flow__viewport') as HTMLElement).transform;
      // With focus on the canvas itself and no node, the arrow keys pan.
      await env.reach(() => env.focus(env.root.querySelector<HTMLElement>('[data-lod]') as HTMLElement));
      await env.unit('pan', 1, async () => {
        await env.keys('{ArrowLeft}');
      });
      await waitFor(() => expect(getComputedStyle(env.root.querySelector('.react-flow__viewport') as HTMLElement).transform).not.toBe(before));
    },
    verify: async (env) => {
      expect(env.root.querySelector('.react-flow__viewport')).not.toBeNull();
    },
  },

  // P-25: rename a node in place.
  'P-25': {
    frame: { graph: pair, ...fast },
    pointer: async (env) => {
      const target = await env.node('HTTP Request');
      await env.unit('rename', 2, async () => {
        await env.dblClick(target.querySelector('[data-node-name]') as HTMLElement);
        const field = await env.q.findByRole('textbox', { name: 'Node name' });
        await env.type(field, 'Inventory');
        await env.keys('{Enter}');
      });
    },
    keyboard: async (env) => {
      await env.reach(async () => env.focus(await env.node('HTTP Request')));
      await env.unit('rename', 2, async () => {
        await env.keys('{F2}');
        const field = await env.q.findByRole('textbox', { name: 'Node name' });
        await env.type(field, 'Inventory');
        await env.keys('{Enter}');
      });
    },
    verify: async (env) => {
      await waitFor(() => expect(env.getGraph().nodes.find((node) => node.id === 'http')?.name).toMatch(/Inventory$/));
    },
  },

  // P-27: find a node type in the palette.
  'P-27': {
    frame: { graph: pair, ...fast },
    pointer: async (env) => {
      await env.unit('find and add', 2, async () => {
        await env.click(await env.button('Add node'));
        const field = await env.q.findByRole('combobox', { name: 'Search nodes' });
        await env.type(field, 'email');
        await env.click(await env.option(/Send email/));
      });
    },
    keyboard: async (env) => {
      await env.reach(async () => env.focus(await env.node('HTTP Request')));
      await env.unit('find and add', 2, async () => {
        await env.keys('a');
        await pickWithKeyboard(env, 'email');
      });
    },
    verify: async (env) => {
      await waitFor(() => expect(byName(env.getGraph(), 'Send email')).toBeDefined());
    },
  },

  // P-28: a node with several outputs labels them inside the tile and grows past two.
  'P-28': {
    frame: {
      graph: {
        nodes: [
          place(exampleNodes.validate, 40, 160),
          {
            ...place(exampleNodes.validate, 340, 140),
            id: 'switch',
            name: 'Route order',
            subtitle: 'Switch on country',
            logicType: 'switch',
            outputs: ['CO', 'MX', 'PE', 'CL'].map((id) => ({ id, label: id })),
          },
        ],
        edges: [],
      },
      ...fast,
    },
    pointer: async (env) => {
      await env.unit('read the nodes', 0, async () => undefined);
    },
    keyboard: async (env) => {
      await env.reach(async () => env.focus(await env.node('Route order')));
      await env.unit('read the nodes', 0, async () => undefined);
    },
    verify: async (env) => {
      const branch = await env.node('Validate order');
      expect(Array.from(branch.querySelectorAll('[data-output-label]')).map((label) => label.textContent)).toEqual(['true', 'false']);
      const route = await env.node('Route order');
      expect(Array.from(route.querySelectorAll('[data-output-label]')).map((label) => label.textContent)).toEqual(['CO', 'MX', 'PE', 'CL']);
      // max(100, 26 x 4 + 22) = 126
      expect(route.querySelector<HTMLElement>('[data-canvas-node]')?.getBoundingClientRect().height).toBeGreaterThan(branch.querySelector<HTMLElement>('[data-canvas-node]')?.getBoundingClientRect().height ?? 0);
    },
  },

  // P-29: the trigger node, and its Execute workflow action.
  'P-29': {
    frame: { graph: chain, ...fast },
    pointer: async (env) => {
      const target = await env.node('Webhook');
      await env.hover(target);
      const execute = await waitFor(() => {
        const element = target.querySelector<HTMLElement>('[data-node-toolbar] [aria-label="Execute workflow"]');
        expect(element).not.toBeNull();
        return element as HTMLElement;
      });
      await env.unit('execute from the trigger', 1, async () => {
        await env.click(execute);
      });
    },
    keyboard: async (env) => {
      const target = await env.node('Webhook');
      await env.reach(() => env.focus(target));
      const toolbarButton = await waitFor(() => {
        const element = target.querySelector<HTMLElement>('[data-node-toolbar] [aria-label="Execute workflow"]');
        expect(element).not.toBeNull();
        return element as HTMLElement;
      });
      await env.reach(() => env.focus(toolbarButton));
      await env.unit('execute from the trigger', 1, async () => {
        await env.keys('{Enter}');
      });
    },
    verify: async (env) => {
      for (const id of ['webhook', 'http', 'slack']) await expectCompleted(env, id);
      const target = await env.node('Webhook');
      const surface = target.querySelector('[data-tile-surface]') as HTMLElement;
      // A half circle on the left, with a lightning mark and no input port.
      expect(getComputedStyle(surface).borderTopLeftRadius).toBe('50px');
      expect(target.querySelector('[data-trigger-mark]')).not.toBeNull();
      expect(target.querySelector('[data-port="input"]')).toBeNull();
    },
  },

  // P-30: the same, by keyboard (and in dark when the theme is dark): open a node, switch the data view, go back.
  'P-30': {
    frame: { graph: chain, outputs: { webhook: { main: orderItems }, http: { main: orderItems } }, ...fast },
    keyboard: async (env) => {
      await env.reach(async () => env.focus(await env.node('HTTP Request')));
      await env.unit('open the node', 1, async () => {
        await env.keys('{Enter}');
      });
      const dialog = await env.q.findByRole('dialog', { name: 'HTTP Request details' });
      const pane = dialog.querySelector('[data-data-pane="output"]') as HTMLElement;
      const schema = Array.from(pane.querySelectorAll('button')).find((button) => button.textContent === 'Schema') as HTMLElement;
      await env.reach(() => env.focus(schema));
      await env.unit('show the schema', 1, async () => {
        await env.keys('{Enter}');
      });
    },
    verify: async (env) => {
      const dialog = await env.q.findByRole('dialog', { name: 'HTTP Request details' });
      await waitFor(() => expect(dialog.querySelector('[data-data-pane="output"] [data-schema-view]')).not.toBeNull());
    },
  },
};

