// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { act, cleanup, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { page } from '@vitest/browser/context';
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';

import '../../src/styles/index.css';
import { exampleCatalog, exampleDraft, exampleGraph } from '../../src/canvas/fixtures';
import type { Graph } from '../../src/canvas/model';
import { WorkflowCanvas, type WorkflowCanvasProps } from '../../src/canvas/WorkflowCanvas';
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
  const view = render(
    <Providers theme="light" density="comfortable" locale="en">
      <div className="h-screen">
        <WorkflowCanvas defaultGraph={exampleDraft} catalog={exampleCatalog} onGraphChange={onGraphChange} {...props} />
      </div>
    </Providers>,
  );
  return { ...view, onGraphChange, user: userEvent.setup() };
}

const node = (name: RegExp | string) => screen.findByRole('group', { name: typeof name === 'string' ? new RegExp(name) : name });
const last = (mock: ReturnType<typeof vi.fn<(graph: Graph) => void>>) => mock.mock.calls.at(-1)![0];
const position = (graph: Graph, id: string) => graph.nodes.find((candidate) => candidate.id === id)!.position;

// user-event mouse events carry no window, which React Flow's drag handling needs, so a node is focused directly.
async function focusNode(_user: ReturnType<typeof userEvent.setup>, name: string) {
  const element = await node(name);
  act(() => element.focus());
  return element;
}

describe('the canvas is named, labelled and has one tab stop', () => {
  it('gives every node the accessible name "<kind> node <name>, <summary>, status, inputs, outputs"', async () => {
    renderCanvas({ defaultGraph: exampleGraph });
    const charge = await node('Charge card');
    expect(charge).toHaveAccessibleName(
      'Action node Charge card, Charges the customer card for the total of the order., status Succeeded with errors, 1 input, 1 output',
    );
    expect(await node('In stock')).toHaveAccessibleName(/^Control node In stock\?, status Succeeded, 1 input, 2 outputs$/);
    expect(await node('Order received')).toHaveAccessibleName(/^Trigger node Order received/);
  });

  it('puts exactly one node in the tab order and moves it with the arrow keys', async () => {
    const { user } = renderCanvas();
    await node('Order received');
    const tabbable = () => Array.from(document.querySelectorAll<HTMLElement>('.react-flow__node')).filter((element) => element.tabIndex === 0);
    expect(tabbable()).toHaveLength(1);
    const first = await focusNode(user, 'Order received');
    await user.keyboard('{ArrowRight}');
    await waitFor(() => expect(document.activeElement).toBe(screen.getByRole('group', { name: /Check stock/ })));
    expect(first).not.toBe(document.activeElement);
    await waitFor(() => expect(tabbable()).toHaveLength(1));
    expect(tabbable()[0]).toBe(document.activeElement);
  });
});

describe('every graph operation works from the keyboard', () => {
  it('moves a node one grid step with Shift + arrow and four with Alt + Shift + arrow', async () => {
    const { user, onGraphChange } = renderCanvas();
    await focusNode(user, 'Check stock');
    const start = position(exampleDraft, 'check-stock');
    await user.keyboard('{Shift>}{ArrowRight}{/Shift}');
    expect(position(last(onGraphChange), 'check-stock')).toEqual({ x: start.x + 8, y: start.y });
    await user.keyboard('{Alt>}{Shift>}{ArrowDown}{/Shift}{/Alt}');
    expect(position(last(onGraphChange), 'check-stock')).toEqual({ x: start.x + 8, y: start.y + 32 });
  });

  it('adds a node: A opens the searchable list, typing filters, Enter adds', async () => {
    const { user, onGraphChange } = renderCanvas();
    await focusNode(user, 'Check stock');
    await user.keyboard('a');
    const search = await screen.findByRole('combobox', { name: 'Search nodes' });
    await user.type(search, 'email');
    await user.keyboard('{Enter}');
    await waitFor(() => expect(onGraphChange).toHaveBeenCalled());
    const added = last(onGraphChange).nodes.find((candidate) => candidate.name === 'Send email');
    expect(added).toBeDefined();
    expect(last(onGraphChange).nodes).toHaveLength(exampleDraft.nodes.length + 1);
    expect(added!.position.x % 8).toBe(0);
    // The new node is announced politely and receives focus.
    await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('Added Send email'));
    await waitFor(() => expect(document.activeElement).toBe(screen.getByRole('group', { name: /Send email/ })));
  });

  it('connects two nodes from lists: C opens the form', async () => {
    const { user, onGraphChange } = renderCanvas();
    await focusNode(user, 'Order received');
    await user.keyboard('c');
    const dialog = await screen.findByRole('dialog', { name: 'Connect nodes' });
    // The focused node is the source; its only output is chosen for us.
    await user.click(within(dialog).getByRole('combobox', { name: 'To node' }));
    await user.click(await screen.findByRole('option', { name: 'Send receipt' }));
    await user.click(within(dialog).getByRole('button', { name: 'Connect' }));
    await waitFor(() => expect(onGraphChange).toHaveBeenCalled());
    const edges = last(onGraphChange).edges;
    expect(edges).toHaveLength(exampleDraft.edges.length + 1);
    expect(edges.at(-1)).toMatchObject({ source: 'order-received', target: 'send-receipt', sourceOutput: 'main', targetInput: 'main' });
    await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('Connected Order received to Send receipt'));
  });

  it('refuses a connection to itself with a message next to the field', async () => {
    const { user, onGraphChange } = renderCanvas();
    await focusNode(user, 'Check stock');
    await user.keyboard('c');
    const dialog = await screen.findByRole('dialog', { name: 'Connect nodes' });
    await user.click(within(dialog).getByRole('combobox', { name: 'To node' }));
    await user.click(await screen.findByRole('option', { name: 'Check stock' }));
    await user.click(within(dialog).getByRole('button', { name: 'Connect' }));
    expect(await within(dialog).findByText('A node cannot be connected to itself.')).toBeVisible();
    expect(onGraphChange).not.toHaveBeenCalled();
  });

  it('deletes one node at once with an Undo notice, and Ctrl + Z brings it back', async () => {
    const { user, onGraphChange } = renderCanvas();
    await focusNode(user, 'Send receipt');
    await user.keyboard('{Delete}');
    await waitFor(() => expect(last(onGraphChange).nodes.some((candidate) => candidate.id === 'send-receipt')).toBe(false));
    // Connections of the deleted node go with it.
    expect(last(onGraphChange).edges.every((edge) => edge.source !== 'send-receipt' && edge.target !== 'send-receipt')).toBe(true);
    await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('Deleted 1 node'));
    // Focus stays inside the canvas: it moves to a remaining node.
    await waitFor(() => expect(document.activeElement?.classList.contains('react-flow__node')).toBe(true));
    await user.keyboard('{Control>}z{/Control}');
    await waitFor(() => expect(last(onGraphChange).nodes.some((candidate) => candidate.id === 'send-receipt')).toBe(true));
    await user.keyboard('{Control>}{Shift>}z{/Shift}{/Control}');
    await waitFor(() => expect(last(onGraphChange).nodes.some((candidate) => candidate.id === 'send-receipt')).toBe(false));
  });

  it('asks before deleting several nodes, and says it can be undone', async () => {
    const { user, onGraphChange } = renderCanvas();
    await focusNode(user, 'Check stock');
    await user.keyboard(' ');
    await user.keyboard('{ArrowRight}');
    await waitFor(() => expect(document.activeElement).toBe(screen.getByRole('group', { name: /In stock/ })));
    await user.keyboard(' ');
    await user.keyboard('{Delete}');
    const dialog = await screen.findByRole('alertdialog', { name: 'Delete 2 nodes?' });
    expect(dialog).toHaveTextContent('You can undo this right after.');
    expect(onGraphChange).not.toHaveBeenCalled();
    await user.click(within(dialog).getByRole('button', { name: 'Delete nodes' }));
    await waitFor(() => expect(last(onGraphChange).nodes).toHaveLength(exampleDraft.nodes.length - 2));
  });

  it('opens the details with Enter through the host callback', async () => {
    const onInspect = vi.fn();
    const { user } = renderCanvas({ onInspect });
    await focusNode(user, 'Check stock');
    await user.keyboard('{Enter}');
    expect(onInspect).toHaveBeenCalledWith('check-stock');
  });

  it('lists the shortcuts with ?', async () => {
    const { user } = renderCanvas();
    await focusNode(user, 'Check stock');
    await user.keyboard('?');
    const dialog = await screen.findByRole('dialog', { name: 'Keyboard shortcuts' });
    expect(within(dialog).getByText('Add a node')).toBeVisible();
    expect(within(dialog).getByText('Connect two nodes')).toBeVisible();
  });

  it('does not react to shortcuts typed in a field', async () => {
    const { user } = renderCanvas();
    await focusNode(user, 'Check stock');
    await user.keyboard('a');
    const search = await screen.findByRole('combobox', { name: 'Search nodes' });
    await user.type(search, 'c?');
    expect(screen.queryByRole('dialog', { name: 'Connect nodes' })).toBeNull();
    expect(screen.queryByRole('dialog', { name: 'Keyboard shortcuts' })).toBeNull();
  });
});

describe('the toolbar and the outline are keyboard paths too', () => {
  it('every graph operation has a toolbar button', async () => {
    renderCanvas();
    const toolbar = await screen.findByRole('toolbar', { name: 'Canvas tools' });
    for (const name of ['Add node', 'Connect nodes', 'Delete selected', 'Undo', 'Redo', 'Flow outline', 'Keyboard shortcuts']) {
      expect(within(toolbar).getByRole('button', { name })).toBeInTheDocument();
    }
  });

  it('the outline describes the graph and can focus a node, remove a connection and delete a node', async () => {
    const { user, onGraphChange } = renderCanvas({ defaultOutlineOpen: true });
    const outline = await screen.findByRole('region', { name: 'Flow outline' });
    expect(within(outline).getByText('6 nodes and 6 connections')).toBeVisible();
    expect(within(outline).getByText('Output true goes to Charge card, input main')).toBeVisible();

    await user.click(within(outline).getByRole('button', { name: 'Go to Charge card' }));
    await waitFor(() => expect(document.activeElement).toBe(screen.getByRole('group', { name: /Charge card/ })));

    await user.click(within(outline).getByRole('button', { name: 'Remove connection from Order received to Check stock' }));
    await waitFor(() => expect(last(onGraphChange).edges.some((edge) => edge.id === 'e1')).toBe(false));
    await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('Removed the connection from Order received to Check stock'));

    await user.click(within(outline).getAllByRole('button', { name: /^Delete / })[0]!);
    await waitFor(() => expect(last(onGraphChange).nodes).toHaveLength(exampleDraft.nodes.length - 1));
  });
});

describe('read-only canvas (below md, or by choice)', () => {
  it('renders no editing affordance and ignores editing keys', async () => {
    const { user, onGraphChange } = renderCanvas({ readOnly: true, defaultGraph: exampleGraph });
    await focusNode(user, 'Check stock');
    expect(screen.queryByRole('button', { name: 'Add node' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Delete selected' })).toBeNull();
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
    await focusNode(user, 'Check stock');
    await user.keyboard('{Enter}');
    const panel = await screen.findByRole('dialog', { name: 'Check stock' });
    expect(within(panel).getByText('Succeeded')).toBeVisible();
    expect(within(panel).getByText('15 items received')).toBeVisible();
    expect(within(panel).getByText('12 items sent on')).toBeVisible();
    expect(within(panel).getByRole('link', { name: 'Open the execution detail' })).toHaveAttribute('href', '/executions/example/check-stock');
  });

  it('is view only on a phone-width viewport even when editing is allowed', async () => {
    await page.viewport(390, 844);
    try {
      renderCanvas();
      await node('Check stock');
      expect(screen.queryByRole('button', { name: 'Add node' })).toBeNull();
      expect(screen.getByText(/View only/)).toBeVisible();
    } finally {
      await page.viewport(1440, 900);
    }
  });
});

describe('execution overlay', () => {
  it('shows the state of a node as icon and text and announces a change politely', async () => {
    const view = renderCanvas({ defaultGraph: exampleGraph, statuses: { 'send-receipt': { status: 'NOT_RUN' } } });
    await node('Send receipt');
    view.rerender(
      <Providers theme="light" density="comfortable" locale="en">
        <div className="h-screen">
          <WorkflowCanvas defaultGraph={exampleGraph} statuses={{ 'send-receipt': { status: 'RUNNING' } }} />
        </div>
      </Providers>,
    );
    await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('Send receipt is now Running'));
    const card = (await node('Send receipt')).querySelector('[data-node-status]');
    expect(card?.querySelector('svg')).not.toBeNull();
    expect(card).toHaveTextContent('Running');
  });
});

describe('the canvas opens fitted to the flow with readable text', () => {
  const zoomOf = (container: HTMLElement) => {
    const viewport = container.querySelector<HTMLElement>('.react-flow__viewport');
    return Number(/matrix\(([\d.]+)/.exec(getComputedStyle(viewport!).transform)?.[1] ?? 'NaN');
  };
  const inside = (inner: DOMRect, outer: DOMRect) =>
    inner.left >= outer.left && inner.top >= outer.top && inner.right <= outer.right && inner.bottom <= outer.bottom;

  it('keeps the trigger on screen when the view is too narrow to show the whole flow (phone)', async () => {
    await page.viewport(390, 844);
    try {
      const { container } = renderCanvas({ defaultGraph: exampleGraph });
      const trigger = await node('Order received');
      const region = container.querySelector<HTMLElement>('[role="region"]')!;
      await waitFor(() => expect(inside(trigger.getBoundingClientRect(), region.getBoundingClientRect())).toBe(true));
      // The zoom never drops below the readable minimum to make everything fit.
      expect(zoomOf(container)).toBeGreaterThanOrEqual(0.6 - 0.01);
    } finally {
      await page.viewport(1440, 900);
    }
  });

  it('shows the whole flow when it fits, and never below the readable zoom on a tablet', async () => {
    await page.viewport(1024, 768);
    try {
      const { container } = renderCanvas({ defaultGraph: exampleGraph });
      const trigger = await node('Order received');
      const region = container.querySelector<HTMLElement>('[role="region"]')!;
      await waitFor(() => expect(inside(trigger.getBoundingClientRect(), region.getBoundingClientRect())).toBe(true));
      expect(zoomOf(container)).toBeGreaterThanOrEqual(0.75 - 0.01);
    } finally {
      await page.viewport(1440, 900);
    }
  });

  it('fits the whole flow on a wide screen', async () => {
    const { container } = renderCanvas({ defaultGraph: exampleGraph });
    await node('Order received');
    const region = container.querySelector<HTMLElement>('[role="region"]')!;
    await waitFor(() => {
      const nodes = Array.from(container.querySelectorAll<HTMLElement>('.react-flow__node'));
      expect(nodes.length).toBe(exampleGraph.nodes.length);
      for (const element of nodes) expect(inside(element.getBoundingClientRect(), region.getBoundingClientRect())).toBe(true);
    });
  });
});
