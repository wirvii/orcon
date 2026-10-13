// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { act, cleanup, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { page } from '@vitest/browser/context';
import { createRef } from 'react';
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';

import '../../src/styles/index.css';
import { exampleCatalog, exampleDraft, exampleGraph } from '../../src/canvas/fixtures';
import type { Graph } from '../../src/canvas/model';
import { WorkflowCanvas, type WorkflowCanvasHandle, type WorkflowCanvasProps } from '../../src/canvas/WorkflowCanvas';
import { Providers } from '../../src/providers/Providers';

beforeAll(async () => {
  await page.viewport(1440, 900);
});

afterEach(() => {
  cleanup();
  document.documentElement.removeAttribute('data-theme');
  document.documentElement.removeAttribute('data-density');
});

function renderCanvas(props: Partial<WorkflowCanvasProps> = {}) {
  const onGraphChange = vi.fn<(graph: Graph) => void>();
  const handle = createRef<WorkflowCanvasHandle>();
  const view = render(
    <Providers theme="light" density="comfortable" locale="en">
      <div className="h-screen">
        <WorkflowCanvas ref={handle} defaultGraph={exampleDraft} catalog={exampleCatalog} onGraphChange={onGraphChange} tools {...props} />
      </div>
    </Providers>,
  );
  return { ...view, onGraphChange, handle, user: userEvent.setup() };
}

// Node names start with the node name and a comma; connections (also groups) start with it and a parenthesis.
const nodeName = (name: string) => new RegExp(`^${name}, `);
const node = (name: string) => screen.findByRole('group', { name: nodeName(name) });
const last = (mock: ReturnType<typeof vi.fn<(graph: Graph) => void>>) => mock.mock.calls.at(-1)![0];
const position = (graph: Graph, id: string) => graph.nodes.find((candidate) => candidate.id === id)!.position;

// user-event mouse events carry no window, which React Flow's drag handling needs, so a node is focused directly.
async function focusNode(name: string) {
  const element = await node(name);
  act(() => element.focus());
  return element;
}

describe('the canvas is named, labelled and has one tab stop', () => {
  it('gives every node the accessible name "<name>, <operation>. Status: <status>. <n> inputs, <m> outputs."', async () => {
    renderCanvas({ defaultGraph: exampleGraph });
    expect(await node('HTTP Request')).toHaveAccessibleName('HTTP Request, GET inventory. Status: Running. 1 input, 1 output.');
    expect(await node('Validate order')).toHaveAccessibleName('Validate order, If total > 0. Status: Succeeded. 1 input, 2 outputs.');
    expect(await node('Merge')).toHaveAccessibleName('Merge, Append. Status: Not executed, disabled. 2 inputs, 1 output.');
    expect(await node('Webhook')).toHaveAccessibleName('Webhook, On POST /orders. Status: Succeeded. 0 inputs, 1 output.');
  });

  it('puts exactly one node in the tab order and moves it with the arrow keys', async () => {
    const { user } = renderCanvas();
    await node('Webhook');
    const tabbable = () => Array.from(document.querySelectorAll<HTMLElement>('.react-flow__node')).filter((element) => element.tabIndex === 0);
    expect(tabbable()).toHaveLength(1);
    await focusNode('Webhook');
    await user.keyboard('{ArrowRight}');
    await waitFor(() => expect(document.activeElement).toBe(screen.getByRole('group', { name: nodeName('Validate order') })));
    await waitFor(() => expect(tabbable()).toHaveLength(1));
    expect(tabbable()[0]).toBe(document.activeElement);
  });

  it('offers the toolbar and the output plus buttons only on the node that holds the tab stop', async () => {
    renderCanvas();
    await focusNode('Webhook');
    const toolbar = await screen.findByRole('toolbar', { name: 'Actions for Webhook' });
    for (const name of ['Execute workflow', 'Run step', 'Disable node', 'Delete node', 'More actions']) {
      expect(within(toolbar).getByRole('button', { name })).toHaveProperty('tabIndex', 0);
    }
    expect(screen.getByRole('button', { name: 'Add node after Webhook' })).toHaveProperty('tabIndex', 0);
    // Another node's plus button is not in the tab order.
    expect(screen.getByRole('button', { name: 'Add node after Slack' })).toHaveProperty('tabIndex', -1);
  });
});

describe('every graph operation works from the keyboard', () => {
  it('moves a node one grid step (20) with Shift + arrow and four with Alt + Shift + arrow', async () => {
    const { user, onGraphChange } = renderCanvas();
    await focusNode('Validate order');
    const start = position(exampleDraft, 'validate');
    await user.keyboard('{Shift>}{ArrowRight}{/Shift}');
    expect(position(last(onGraphChange), 'validate')).toEqual({ x: start.x + 20, y: start.y });
    await user.keyboard('{Alt>}{Shift>}{ArrowDown}{/Shift}{/Alt}');
    expect(position(last(onGraphChange), 'validate')).toEqual({ x: start.x + 20, y: start.y + 80 });
  });

  it('adds a node after the focused one: A opens the palette, Enter inserts it one column right, connected and selected', async () => {
    const { user, onGraphChange } = renderCanvas();
    await focusNode('Merge');
    await user.keyboard('a');
    const search = await screen.findByRole('combobox', { name: 'Search nodes' });
    await waitFor(() => expect(search).toHaveFocus());
    await user.type(search, 'email');
    await user.keyboard('{Enter}');
    await waitFor(() => expect(onGraphChange).toHaveBeenCalled());
    const graph = last(onGraphChange);
    const added = graph.nodes.find((candidate) => candidate.name === 'Send email')!;
    const merge = position(exampleDraft, 'merge');
    expect(added.position).toEqual({ x: merge.x + 200, y: merge.y });
    expect(graph.edges.some((edge) => edge.source === 'merge' && edge.target === added.id)).toBe(true);
    await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('Added Send email'));
    await waitFor(() => expect(document.activeElement).toBe(screen.getByRole('group', { name: nodeName('Send email') })));
  });

  it('connects two nodes from lists: C opens the form', async () => {
    const { user, onGraphChange } = renderCanvas();
    await focusNode('Webhook');
    await user.keyboard('c');
    const dialog = await screen.findByRole('dialog', { name: 'Connect nodes' });
    await user.click(within(dialog).getByRole('combobox', { name: 'To node' }));
    await user.click(await screen.findByRole('option', { name: 'Gmail' }));
    await user.click(within(dialog).getByRole('button', { name: 'Connect' }));
    await waitFor(() => expect(onGraphChange).toHaveBeenCalled());
    expect(last(onGraphChange).edges.at(-1)).toMatchObject({ source: 'webhook', target: 'gmail', sourceOutput: 'main', targetInput: 'main' });
    await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('Connected Webhook to Gmail'));
  });

  it('refuses a connection to itself with a message next to the field', async () => {
    const { user, onGraphChange } = renderCanvas();
    await focusNode('HTTP Request');
    await user.keyboard('c');
    const dialog = await screen.findByRole('dialog', { name: 'Connect nodes' });
    await user.click(within(dialog).getByRole('combobox', { name: 'To node' }));
    await user.click(await screen.findByRole('option', { name: 'HTTP Request' }));
    await user.click(within(dialog).getByRole('button', { name: 'Connect' }));
    expect(await within(dialog).findByText('A node cannot be connected to itself.')).toBeVisible();
    expect(onGraphChange).not.toHaveBeenCalled();
  });

  it('deletes one node at once with an Undo notice, and Ctrl + Z brings it back', async () => {
    const { user, onGraphChange } = renderCanvas();
    await focusNode('Gmail');
    await user.keyboard('{Delete}');
    await waitFor(() => expect(last(onGraphChange).nodes.some((candidate) => candidate.id === 'gmail')).toBe(false));
    expect(last(onGraphChange).edges.every((edge) => edge.source !== 'gmail' && edge.target !== 'gmail')).toBe(true);
    await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('Deleted 1 node'));
    await waitFor(() => expect(document.activeElement?.classList.contains('react-flow__node')).toBe(true));
    await user.keyboard('{Control>}z{/Control}');
    await waitFor(() => expect(last(onGraphChange).nodes.some((candidate) => candidate.id === 'gmail')).toBe(true));
    await user.keyboard('{Control>}{Shift>}z{/Shift}{/Control}');
    await waitFor(() => expect(last(onGraphChange).nodes.some((candidate) => candidate.id === 'gmail')).toBe(false));
  });

  it('asks before deleting several nodes, and says it can be undone', async () => {
    const { user, onGraphChange } = renderCanvas();
    await focusNode('Webhook');
    await user.keyboard(' ');
    await user.keyboard('{ArrowRight}');
    await waitFor(() => expect(document.activeElement).toBe(screen.getByRole('group', { name: nodeName('Validate order') })));
    await user.keyboard(' ');
    await user.keyboard('{Delete}');
    const dialog = await screen.findByRole('alertdialog', { name: 'Delete 2 nodes?' });
    expect(dialog).toHaveTextContent('You can undo this right after.');
    expect(onGraphChange).not.toHaveBeenCalled();
    await user.click(within(dialog).getByRole('button', { name: 'Delete nodes' }));
    await waitFor(() => expect(last(onGraphChange).nodes).toHaveLength(exampleDraft.nodes.length - 2));
  });

  it('disables a node with D, pins its output with P and renames it with F2', async () => {
    const { user, onGraphChange } = renderCanvas();
    await focusNode('HTTP Request');
    await user.keyboard('d');
    expect(last(onGraphChange).nodes.find((candidate) => candidate.id === 'http')?.disabled).toBe(true);
    await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('HTTP Request is disabled'));
    await user.keyboard('p');
    expect(last(onGraphChange).nodes.find((candidate) => candidate.id === 'http')?.pinned).toBe(true);
    await user.keyboard('{F2}');
    const field = await screen.findByRole('textbox', { name: 'Node name' });
    await user.clear(field);
    await user.type(field, 'Inventory{Enter}');
    expect(last(onGraphChange).nodes.find((candidate) => candidate.id === 'http')?.name).toBe('Inventory');
  });

  it('adds a sticky note with Shift + S', async () => {
    const { user, onGraphChange } = renderCanvas();
    await focusNode('Webhook');
    await user.keyboard('{Shift>}s{/Shift}');
    await waitFor(() => expect(last(onGraphChange).notes).toHaveLength(2));
    await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('Added a sticky note'));
  });

  it('opens the details with Enter through the host callback', async () => {
    const onOpenDetails = vi.fn();
    const { user } = renderCanvas({ onOpenDetails });
    await focusNode('HTTP Request');
    await user.keyboard('{Enter}');
    expect(onOpenDetails).toHaveBeenCalledWith('http', undefined);
  });

  it('lists the shortcuts with ?', async () => {
    const { user } = renderCanvas();
    await focusNode('HTTP Request');
    await user.keyboard('?');
    const dialog = await screen.findByRole('dialog', { name: 'Keyboard shortcuts' });
    expect(within(dialog).getByText('Add a node')).toBeVisible();
    expect(within(dialog).getByText('Connect two nodes')).toBeVisible();
    expect(within(dialog).getByText('Add a sticky note')).toBeVisible();
  });

  it('does not react to shortcuts typed in a field', async () => {
    const { user } = renderCanvas();
    await focusNode('HTTP Request');
    await user.keyboard('a');
    const search = await screen.findByRole('combobox', { name: 'Search nodes' });
    await user.type(search, 'c?');
    expect(screen.queryByRole('dialog', { name: 'Connect nodes' })).toBeNull();
    expect(screen.queryByRole('dialog', { name: 'Keyboard shortcuts' })).toBeNull();
  });
});

describe('the tools and the outline are keyboard paths too', () => {
  it('every graph operation has a button in the tools strip', async () => {
    renderCanvas();
    const toolbar = await screen.findByRole('toolbar', { name: 'Canvas tools' });
    for (const name of ['Add node', 'Connect nodes', 'Delete selected', 'Undo', 'Redo', 'Flow outline', 'Keyboard shortcuts']) {
      expect(within(toolbar).getByRole('button', { name })).toBeInTheDocument();
    }
  });

  it('the outline describes the graph, connection states included, and can focus a node, remove a connection and delete a node', async () => {
    const { user, onGraphChange } = renderCanvas({ defaultOutlineOpen: true, defaultGraph: exampleGraph });
    const outline = await screen.findByRole('region', { name: 'Flow outline' });
    expect(within(outline).getByText('6 nodes and 6 connections')).toBeVisible();
    expect(within(outline).getByText('Output true goes to HTTP Request, input main, 1 item, succeeded')).toBeVisible();
    expect(within(outline).getByText('Output main goes to Merge, input a, uncertain')).toBeVisible();

    await user.click(within(outline).getByRole('button', { name: 'Go to Gmail' }));
    await waitFor(() => expect(document.activeElement).toBe(screen.getByRole('group', { name: nodeName('Gmail') })));

    await user.click(within(outline).getByRole('button', { name: 'Remove connection from Webhook to Validate order' }));
    await waitFor(() => expect(last(onGraphChange).edges.some((edge) => edge.id === 'e1')).toBe(false));
    await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('Removed the connection from Webhook to Validate order'));

    await user.click(within(outline).getAllByRole('button', { name: /^Delete / })[0]!);
    await waitFor(() => expect(last(onGraphChange).nodes).toHaveLength(exampleGraph.nodes.length - 1));
  });
});

describe('connections are focusable and have the same actions as the bar', () => {
  it('Delete on a focused connection removes it, Enter opens the palette to insert a node on it', async () => {
    const { user, onGraphChange } = renderCanvas();
    await node('Webhook');
    const edge = await waitFor(() => {
      const element = document.querySelector<HTMLElement>('.react-flow__edge[data-id="e1"]');
      expect(element).not.toBeNull();
      return element as HTMLElement;
    });
    expect(edge).toHaveAccessibleName('Webhook (main) to Validate order');
    act(() => edge.focus());
    await user.keyboard('{Enter}');
    const search = await screen.findByRole('combobox', { name: 'Search nodes' });
    await waitFor(() => expect(search).toHaveFocus());
    await user.keyboard('{Enter}');
    await waitFor(() => expect(onGraphChange).toHaveBeenCalled());
    const graph = last(onGraphChange);
    expect(graph.edges.some((candidate) => candidate.id === 'e1')).toBe(false);
    const added = graph.nodes.find((candidate) => !exampleDraft.nodes.some((existing) => existing.id === candidate.id))!;
    expect(graph.edges.some((candidate) => candidate.source === 'webhook' && candidate.target === added.id)).toBe(true);
    expect(graph.edges.some((candidate) => candidate.source === added.id && candidate.target === 'validate')).toBe(true);
  });

  it('Delete removes the focused connection', async () => {
    const { user, onGraphChange } = renderCanvas();
    await node('Webhook');
    const edge = await waitFor(() => document.querySelector<HTMLElement>('.react-flow__edge[data-id="e3"]')!);
    act(() => edge.focus());
    await user.keyboard('{Delete}');
    expect(last(onGraphChange).edges.some((candidate) => candidate.id === 'e3')).toBe(false);
  });
});

describe('read-only canvas (below md, or by choice)', () => {
  it('renders no editing affordance and ignores editing keys', async () => {
    const { user, onGraphChange } = renderCanvas({ readOnly: true, defaultGraph: exampleGraph });
    await focusNode('HTTP Request');
    expect(screen.queryByRole('button', { name: 'Add node' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Delete selected' })).toBeNull();
    expect(screen.queryByRole('button', { name: /^Add node after/ })).toBeNull();
    expect(screen.queryByRole('toolbar', { name: /^Actions for/ })).toBeNull();
    expect(screen.getByText(/View only/)).toBeVisible();
    await user.keyboard('a');
    await user.keyboard('{Shift>}{ArrowRight}{/Shift}');
    await user.keyboard('{Delete}');
    expect(screen.queryByRole('combobox', { name: 'Search nodes' })).toBeNull();
    expect(onGraphChange).not.toHaveBeenCalled();
    // Looking around still works.
    expect(screen.getByRole('button', { name: 'Zoom in' })).toBeVisible();
  });

  it('opens a panel with the state, the counts and the execution link when a node is chosen', async () => {
    const { user } = renderCanvas({ readOnly: true, defaultGraph: exampleGraph, executionHref: (id) => `/executions/example/${id}` });
    await focusNode('HTTP Request');
    await user.keyboard('{Enter}');
    const panel = await screen.findByRole('dialog', { name: 'HTTP Request' });
    expect(within(panel).getByText('Running')).toBeVisible();
    expect(within(panel).getByText('1 item received')).toBeVisible();
    expect(within(panel).getByText('3 items sent on')).toBeVisible();
    expect(within(panel).getByRole('link', { name: 'Open the execution detail' })).toHaveAttribute('href', '/executions/example/http');
  });

  it('is view only on a phone-width viewport even when editing is allowed', async () => {
    await page.viewport(390, 844);
    try {
      renderCanvas();
      await node('HTTP Request');
      expect(screen.queryByRole('button', { name: 'Add node' })).toBeNull();
      expect(screen.getByText(/View only/)).toBeVisible();
    } finally {
      await page.viewport(1440, 900);
    }
  });
});

describe('execution overlay', () => {
  it('shows the state of a node and announces a change politely', async () => {
    const view = renderCanvas({ defaultGraph: exampleDraft, overlay: { nodes: { gmail: { status: 'WAITING' } } } });
    await node('Gmail');
    view.rerender(
      <Providers theme="light" density="comfortable" locale="en">
        <div className="h-screen">
          <WorkflowCanvas defaultGraph={exampleDraft} overlay={{ nodes: { gmail: { status: 'RUNNING' } } }} />
        </div>
      </Providers>,
    );
    await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('Gmail is now Running'));
    const tile = (await node('Gmail')).querySelector('[data-canvas-node]');
    expect(tile?.querySelector('[data-node-chip="RUNNING"]')).toHaveTextContent('Running');
    expect(tile?.querySelector('[data-node-badge="RUNNING"]')).not.toBeNull();
  });
});

describe('the canvas opens fitted to the flow with readable text', () => {
  const zoomOf = (container: HTMLElement) => {
    const viewport = container.querySelector<HTMLElement>('.react-flow__viewport');
    return Number(/matrix\(([\d.]+)/.exec(getComputedStyle(viewport!).transform)?.[1] ?? 'NaN');
  };
  const inside = (inner: DOMRect, outer: DOMRect) => inner.left >= outer.left - 1 && inner.top >= outer.top - 1 && inner.right <= outer.right + 1 && inner.bottom <= outer.bottom + 1;

  it('fits the whole flow on a phone, where the canvas is view only', async () => {
    await page.viewport(390, 844);
    try {
      const { container } = renderCanvas({ defaultGraph: exampleGraph });
      await node('Webhook');
      const region = container.querySelector<HTMLElement>('[role="region"]')!;
      await waitFor(() => {
        const nodes = Array.from(container.querySelectorAll<HTMLElement>('.react-flow__node'));
        expect(nodes.length).toBe(exampleGraph.nodes.length + (exampleGraph.notes?.length ?? 0));
        for (const element of nodes) expect(inside(element.getBoundingClientRect(), region.getBoundingClientRect())).toBe(true);
      });
    } finally {
      await page.viewport(1440, 900);
    }
  });

  it('keeps the trigger on screen when an editable canvas is too narrow to show the whole flow', async () => {
    await page.viewport(800, 844);
    try {
      const { container } = renderCanvas({ defaultGraph: exampleGraph });
      const trigger = await node('Webhook');
      const region = container.querySelector<HTMLElement>('[role="region"]')!;
      await waitFor(() => expect(inside(trigger.getBoundingClientRect(), region.getBoundingClientRect())).toBe(true));
    } finally {
      await page.viewport(1440, 900);
    }
  });

  it('never fits to more than 100 % and shows the whole flow on a wide screen', async () => {
    const { container } = renderCanvas();
    await node('Webhook');
    const region = container.querySelector<HTMLElement>('[role="region"]')!;
    await waitFor(() => {
      const nodes = Array.from(container.querySelectorAll<HTMLElement>('.react-flow__node'));
      expect(nodes.length).toBe(exampleDraft.nodes.length + (exampleDraft.notes?.length ?? 0));
      for (const element of nodes) expect(inside(element.getBoundingClientRect(), region.getBoundingClientRect())).toBe(true);
    });
    expect(zoomOf(container)).toBeLessThanOrEqual(1.001);
  });
});

describe('level of detail', () => {
  it('at 50 % hides subtitles, connection labels and toolbars, and keeps names readable', async () => {
    const { container, handle } = renderCanvas({ defaultGraph: exampleGraph, tools: false });
    await node('HTTP Request');
    expect(container.querySelector('[data-node-subtitle]')).not.toBeNull();
    expect(container.querySelector('[data-edge-label]')).not.toBeNull();
    act(() => handle.current?.zoomTo(0.5));
    await waitFor(() => expect(container.querySelector('[data-lod]')?.getAttribute('data-lod')).toBe('detail'));
    expect(container.querySelector('[data-node-subtitle]')).toBeNull();
    expect(container.querySelector('[data-edge-label]')).toBeNull();
    expect(container.querySelector('[data-node-toolbar]')).toBeNull();
    // The name keeps an on-screen size of at least 10 px: 13 px text scaled by the counter-scale and the zoom.
    const name = container.querySelector<HTMLElement>('[data-node-name]')!;
    const lineHeight = 18;
    const onScreen = (name.getBoundingClientRect().height / lineHeight) * 13;
    expect(onScreen).toBeGreaterThanOrEqual(9.5);
  });

  it('at 30 % hides labels and chips, and keeps tiles, badges and connections', async () => {
    const { container, handle } = renderCanvas({ defaultGraph: exampleGraph, tools: false });
    await node('HTTP Request');
    act(() => handle.current?.zoomTo(0.3));
    await waitFor(() => expect(container.querySelector('[data-lod]')?.getAttribute('data-lod')).toBe('minimal'));
    expect(container.querySelector('[data-node-label]')).toBeNull();
    expect(container.querySelector('[data-node-chips]')).toBeNull();
    expect(container.querySelector('[data-node-badge]')).not.toBeNull();
    expect(container.querySelector('[data-edge-state]')).not.toBeNull();
  });
});

describe('motion', () => {
  it('has no animation anywhere under reduced motion, even with a running node and connection', async () => {
    expect(window.matchMedia('(prefers-reduced-motion: reduce)').matches).toBe(true);
    renderCanvas({ defaultGraph: exampleGraph, tools: false });
    await node('HTTP Request');
    expect(document.querySelector('[data-run-ring]')).not.toBeNull();
    expect(document.querySelector('[data-edge-state="running"]')).not.toBeNull();
    expect(document.getAnimations()).toEqual([]);
  });
});
