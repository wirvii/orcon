// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { cleanup, render, screen, waitFor } from '@testing-library/react';
import { page } from '@vitest/browser/context';
import { Background, ReactFlow, ReactFlowProvider } from '@xyflow/react';
import type { ReactNode } from 'react';
import { afterEach, beforeAll, describe, expect, it } from 'vitest';

import '../../src/styles/index.css';
import { CanvasEdge, type CanvasEdgeData, type CanvasFlowEdge } from '../../src/canvas/canvas-edge/CanvasEdge';
import { CanvasNode } from '../../src/canvas/canvas-node/CanvasNode';
import { EdgeLabel } from '../../src/canvas/edge-label/EdgeLabel';
import { exampleNodes, placeholderNode } from '../../src/canvas/fixtures';
import { nodeTypes, type TileFlowNode } from '../../src/canvas/FlowNodes';
import type { GraphNode } from '../../src/canvas/model';
import { Providers } from '../../src/providers/Providers';
import type { StatusKey } from '../../src/components/status-badge/status-map';
import { must } from '../../src/lib/must';

beforeAll(async () => {
  await page.viewport(1440, 900);
});

afterEach(() => {
  cleanup();
  document.documentElement.removeAttribute('data-theme');
});

const stage = (theme: 'light' | 'dark', children: ReactNode) =>
  render(
    <Providers theme={theme} density="comfortable" locale="en">
      <div className="bg-canvas p-16 pb-16">{children}</div>
    </Providers>,
  );

const plain: GraphNode = { ...exampleNodes.http, status: undefined };
const tile = (container: HTMLElement) => must(container.querySelector<HTMLElement>('[data-canvas-node]'));
const surface = (container: HTMLElement) => must(container.querySelector<HTMLElement>('[data-tile-surface]'));
const rect = (element: Element) => element.getBoundingClientRect();
const px = (value: string) => Number.parseFloat(value);

/** Resolves a token to the color the browser computes, so a test compares colors without writing a hex. */
function tokenColor(name: string): string {
  const probe = document.createElement('i');
  probe.style.color = `var(${name})`;
  document.body.append(probe);
  const color = getComputedStyle(probe).color;
  probe.remove();
  return color;
}

describe.each(['light', 'dark'] as const)('the style A tile in the %s theme', (theme) => {
  it('is 100 x 100 with a radius of 18 and a 1 px border', () => {
    const { container } = stage(theme, <CanvasNode node={plain} labelled editable={false} />);
    const box = rect(tile(container));
    expect(box.width).toBe(100);
    expect(box.height).toBe(100);
    const style = getComputedStyle(surface(container));
    expect(px(style.borderTopLeftRadius)).toBe(18);
    expect(px(style.borderBottomRightRadius)).toBe(18);
    expect(px(style.borderTopWidth)).toBe(1);
  });

  it('puts a 36 px icon in a 40 px box, centered', () => {
    const { container } = stage(theme, <CanvasNode node={plain} labelled editable={false} />);
    const icon = must(tile(container).querySelector('svg'));
    expect(rect(icon).width).toBe(36);
    const outer = rect(tile(container));
    const inner = rect(icon);
    expect(Math.abs(inner.left + inner.width / 2 - (outer.left + outer.width / 2))).toBeLessThan(1);
    expect(Math.abs(inner.top + inner.height / 2 - (outer.top + outer.height / 2))).toBeLessThan(1);
  });

  it('draws a trigger with a half circle on the left, no input port and a 22 px lightning mark on its edge', () => {
    const { container } = stage(theme, <CanvasNode node={{ ...exampleNodes.webhook, status: undefined }} labelled editable={false} />);
    const element = tile(container);
    const style = getComputedStyle(surface(container));
    expect(px(style.borderTopLeftRadius)).toBe(50);
    expect(px(style.borderBottomLeftRadius)).toBe(50);
    expect(px(style.borderTopRightRadius)).toBe(18);
    expect(element.querySelector('[data-port="input"]')).toBeNull();
    const mark = must(element.querySelector<HTMLElement>('[data-trigger-mark]'));
    expect(rect(mark).width).toBe(22);
    // The mark is centered on the left edge: it overlaps the tile by half its width.
    expect(Math.round(rect(element).left - rect(mark).left)).toBe(11);
    expect(getComputedStyle(mark).borderTopWidth).toBe('2px');
  });

  it('places the label 6 px under the tile, at most 150 px wide, with the name and the operation', () => {
    const { container } = stage(theme, <CanvasNode node={{ ...plain, name: 'A very long node name that wraps over two lines and then gets cut off with an ellipsis' }} labelled editable={false} />);
    const label = must(container.querySelector<HTMLElement>('[data-node-label]'));
    expect(Math.round(rect(label).top - rect(tile(container)).bottom)).toBe(6);
    expect(rect(label).width).toBeLessThanOrEqual(150);
    const name = must(container.querySelector<HTMLElement>('[data-node-name]'));
    expect(px(getComputedStyle(name).fontSize)).toBe(13);
    expect(getComputedStyle(name).fontWeight).toBe('600');
    // At most two lines, then an ellipsis.
    expect(rect(name).height).toBeLessThanOrEqual(36.5);
    expect(px(getComputedStyle(must(container.querySelector('[data-node-subtitle]'))).fontSize)).toBe(12);
  });

  it('puts a 22 px status badge on the top-right corner, offset by -8 / -8', () => {
    const { container } = stage(theme, <CanvasNode node={{ ...plain, status: 'FAILED' }} labelled editable={false} />);
    const badge = must(container.querySelector<HTMLElement>('[data-node-badge]'));
    expect(rect(badge).width).toBe(22);
    expect(Math.round(rect(badge).right - rect(tile(container)).right)).toBe(8);
    expect(Math.round(rect(tile(container)).top - rect(badge).top)).toBe(8);
    expect(getComputedStyle(badge).borderTopWidth).toBe('2px');
  });

  it('draws 10 px ports that grow to 24 px with a plus on hover and on focus', async () => {
    const { container } = stage(theme, <CanvasNode node={plain} labelled editable />);
    const input = must(container.querySelector<HTMLElement>('[data-port="input"]'));
    const output = must(container.querySelector<HTMLElement>('[data-port="output"]'));
    const dot = must(output.querySelector<HTMLElement>('[data-port-dot]'));
    expect(rect(input).width).toBe(10);
    expect(rect(dot).width).toBe(10);
    // The control around the dot is at least the target size.
    expect(rect(output).width).toBeGreaterThanOrEqual(24);
    output.focus();
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(Math.round(rect(dot).width)).toBe(24);
    expect(dot.querySelector('svg')).not.toBeNull();
    expect(Number(getComputedStyle(must(dot.querySelector('svg'))).opacity)).toBe(1);
  });

  it('labels the outputs of an if node inside the tile at 24 and 76, in mono text', () => {
    const { container } = stage(theme, <CanvasNode node={{ ...exampleNodes.validate, status: undefined }} labelled editable={false} />);
    const labels = Array.from(container.querySelectorAll<HTMLElement>('[data-output-label]'));
    expect(labels.map((label) => label.textContent)).toEqual(['true', 'false']);
    const outer = rect(tile(container));
    for (const label of labels) {
      expect(rect(label).right).toBeLessThanOrEqual(outer.right);
      expect(rect(label).left).toBeGreaterThanOrEqual(outer.left);
      expect(getComputedStyle(label).fontFamily).toMatch(/mono/i);
      expect(px(getComputedStyle(label).fontSize)).toBe(12);
    }
    const ports = Array.from(container.querySelectorAll<HTMLElement>('[data-port="output"]'));
    expect(ports.map((port) => Math.round(rect(port).top + rect(port).height / 2 - outer.top))).toEqual([24, 76]);
  });

  it('grows to max(100, 26 x outputs + 22) with more than two outputs and keeps the width', () => {
    const many: GraphNode = { ...plain, kind: 'logic', logicType: 'switch', outputs: Array.from({ length: 5 }, (_, index) => ({ id: `o${index}`, label: `case ${index}` })) };
    const { container } = stage(theme, <CanvasNode node={many} labelled editable={false} />);
    expect(rect(tile(container)).height).toBe(152);
    expect(rect(tile(container)).width).toBe(100);
  });

  it('draws the dangling output as a 30 px dashed line ending in a 22 px dashed square with a plus', () => {
    const { container } = stage(theme, <CanvasNode node={plain} labelled editable dangling={['main']} />);
    const stub = must(container.querySelector<HTMLElement>('[data-output-stub]'));
    const line = stub.firstElementChild as HTMLElement;
    expect(rect(line).width).toBe(30);
    expect(getComputedStyle(line).borderTopStyle).toBe('dashed');
    const square = must(stub.querySelector<HTMLElement>('[data-stub-box]'));
    expect(rect(square).width).toBe(22);
    expect(getComputedStyle(square).borderTopStyle).toBe('dashed');
    expect(square.querySelector('svg')).not.toBeNull();
  });

  it('makes the agent tile 200 wide with blocks joined by dashed lines', () => {
    const agent: GraphNode = { ...plain, kind: 'agent', ai: [{ kind: 'model', set: true }, { kind: 'memory', set: false }] };
    const { container } = stage(theme, <CanvasNode node={agent} labelled editable />);
    expect(rect(tile(container)).width).toBe(200);
    const blocks = container.querySelectorAll('[data-ai-block]');
    expect(blocks).toHaveLength(2);
    expect(rect(must(must(blocks[0]).querySelector('button'))).width).toBe(48);
  });

  it('draws a placeholder with a dashed warning border, always disabled, with no run button', () => {
    const { container } = stage(theme, <CanvasNode node={placeholderNode} labelled editable forceToolbar />);
    expect(getComputedStyle(surface(container)).borderTopStyle).toBe('dashed');
    expect(getComputedStyle(surface(container)).borderTopColor).toBe(tokenColor('--orcon-status-warning-fg'));
    expect(container.querySelector('[data-node-chip="not-supported"]')).not.toBeNull();
    expect(screen.queryByRole('button', { name: 'Run step' })).toBeNull();
  });
});

describe('execution states (canvas.md#execution-states)', () => {
  interface Case {
    status: StatusKey;
    border: string;
    borderWidth: number;
    badge: boolean;
    chip: string | null;
  }
  const cases: Case[] = [
    { status: 'RUNNING', border: '--orcon-status-active-fg', borderWidth: 1, badge: true, chip: 'Running' },
    { status: 'SUCCEEDED', border: '--orcon-canvas-node-border', borderWidth: 1, badge: true, chip: null },
    { status: 'SUCCEEDED_WITH_ERRORS', border: '--orcon-status-warning-fg', borderWidth: 2, badge: true, chip: 'Succeeded with errors' },
    { status: 'FAILED', border: '--orcon-status-danger-fg', borderWidth: 2, badge: true, chip: 'Failed' },
    { status: 'UNCERTAIN', border: '--orcon-status-attention-fg', borderWidth: 2, badge: true, chip: 'Needs review' },
    { status: 'WAITING', border: '--orcon-status-active-fg', borderWidth: 2, badge: true, chip: 'Waiting' },
    { status: 'SUSPENDED', border: '--orcon-status-attention-fg', borderWidth: 2, badge: true, chip: 'Needs action' },
    { status: 'CANCELED', border: '--orcon-canvas-node-border', borderWidth: 1, badge: true, chip: 'Canceled' },
    { status: 'NOT_RUN', border: '--orcon-canvas-node-border', borderWidth: 1, badge: false, chip: 'Not run' },
  ];

  it.each(cases)('$status: border, badge and chip', ({ status, border, borderWidth, badge, chip }) => {
    const { container } = stage('light', <CanvasNode node={{ ...plain, status }} labelled editable={false} />);
    const style = getComputedStyle(surface(container));
    expect(px(style.borderTopWidth)).toBe(borderWidth);
    expect(style.borderTopColor).toBe(tokenColor(border));
    expect(container.querySelector('[data-node-badge]') !== null).toBe(badge);
    const chipElement = container.querySelector('[data-node-chips] [data-node-chip]');
    if (chip === null) {
      // Succeeded is the one state without visible text: the quiet state.
      expect(chipElement).toBeNull();
      expect(container.textContent).not.toContain('Succeeded');
    } else {
      expect(chipElement).toHaveTextContent(chip);
      // Every other state shows an icon and text.
      expect(chipElement?.querySelector('svg')).not.toBeNull();
    }
  });

  it('draws a failed node with a danger fill and the error code in the chip', () => {
    const { container } = stage('light', <CanvasNode node={{ ...plain, status: 'FAILED', statusCode: '401' }} labelled editable={false} />);
    expect(getComputedStyle(surface(container)).backgroundColor).toBe(tokenColor('--orcon-status-danger-bg'));
    expect(container.querySelector('[data-node-chips]')).toHaveTextContent('Failed · 401');
  });

  it('draws a disabled node with a dashed border, a sunken fill and a Disabled chip', () => {
    const { container } = stage('light', <CanvasNode node={{ ...plain, disabled: true }} labelled editable={false} />);
    const style = getComputedStyle(surface(container));
    expect(style.borderTopStyle).toBe('dashed');
    expect(style.backgroundColor).toBe(tokenColor('--orcon-surface-sunken'));
    expect(container.querySelector('[data-node-chips]')).toHaveTextContent('Disabled');
    expect(container.querySelector('[data-node-badge]')).toBeNull();
  });

  it('draws the run ring 6 px outside a running node, and only then', () => {
    const { container, rerender } = stage('light', <CanvasNode node={{ ...plain, status: 'RUNNING' }} labelled editable={false} />);
    const ring = must(container.querySelector<HTMLElement>('[data-run-ring]'));
    expect(Math.round(rect(tile(container)).left - rect(ring).left)).toBe(6);
    rerender(
      <Providers theme="light" density="comfortable" locale="en">
        <CanvasNode node={{ ...plain, status: 'SUCCEEDED' }} labelled editable={false} />
      </Providers>,
    );
    expect(document.querySelector('[data-run-ring]')).toBeNull();
  });

  it('draws a pinned node with a pin badge on the top-left corner and a Pinned chip', () => {
    const { container } = stage('light', <CanvasNode node={{ ...plain, status: 'SUCCEEDED', pinned: true }} labelled editable={false} />);
    const badge = must(container.querySelector<HTMLElement>('[data-pinned-badge]'));
    expect(rect(badge).width).toBe(22);
    expect(Math.round(rect(tile(container)).left - rect(badge).left)).toBe(8);
    expect(container.querySelector('[data-node-chips]')).toHaveTextContent('Pinned');
  });

  it('marks a compare mode difference with a border and a chip, never color alone', () => {
    for (const [diff, style, chip] of [
      ['added', 'solid', 'Added'],
      ['changed', 'dashed', 'Changed'],
      ['removed', 'dotted', 'Removed'],
    ] as const) {
      const { container, unmount } = stage('light', <CanvasNode node={{ ...plain, diff }} labelled editable={false} />);
      expect(px(getComputedStyle(surface(container)).borderTopWidth)).toBe(2);
      expect(getComputedStyle(surface(container)).borderTopStyle).toBe(style);
      expect(container.querySelector('[data-node-chips]')).toHaveTextContent(chip);
      unmount();
    }
  });
});

async function findPath(container: HTMLElement, selector: string): Promise<SVGPathElement> {
  return waitFor(() => {
    const element = container.querySelector<SVGPathElement>(selector);
    expect(element).not.toBeNull();
    return element as SVGPathElement;
  });
}

describe('the connection of each state', () => {
  const edgeTypes = { canvas: CanvasEdge };
  function drawEdge(data: CanvasEdgeData) {
    const nodes: TileFlowNode[] = [
      { id: 'a', type: 'tile', position: { x: 0, y: 40 }, data: { node: { ...plain, id: 'a', position: { x: 0, y: 40 } } }, draggable: false },
      { id: 'b', type: 'tile', position: { x: 300, y: 40 }, data: { node: { ...plain, id: 'b', position: { x: 300, y: 40 } } }, draggable: false },
    ];
    const edges: CanvasFlowEdge[] = [{ id: 'e', type: 'canvas', source: 'a', target: 'b', sourceHandle: 'main', targetHandle: 'main', data: { detail: 'full', ...data } }];
    return render(
      <Providers theme="light" density="comfortable" locale="en">
        <ReactFlowProvider>
          <div className="h-screen w-screen">
            <ReactFlow<TileFlowNode, CanvasFlowEdge> nodes={nodes} edges={edges} nodeTypes={nodeTypes} edgeTypes={edgeTypes} fitView>
              <Background />
            </ReactFlow>
          </div>
        </ReactFlowProvider>
      </Providers>,
    );
  }

  const patterns: [string, CanvasEdgeData, string | null, string][] = [
    ['design', {}, null, '--orcon-canvas-edge'],
    ['succeeded', { state: 'succeeded', items: 3 }, null, '--orcon-canvas-edge-active'],
    ['running', { state: 'running', items: 3 }, '7 6', '--orcon-status-active-fg'],
    ['failed', { state: 'failed', items: 0 }, '2 5', '--orcon-status-danger-fg'],
    ['uncertain', { state: 'uncertain' }, '8 5', '--orcon-status-attention-fg'],
    ['not-run', { state: 'not-run', items: 0 }, '3 5', '--orcon-canvas-edge'],
  ];

  it.each(patterns)('%s: stroke color, dash pattern and a 2 px line', async (state, data, dash, color) => {
    const { container } = drawEdge(data);
    const path = await findPath(container, `path[data-edge-state="${state}"]`);
    expect(path.getAttribute('stroke-dasharray')).toBe(dash);
    expect(getComputedStyle(path).stroke).toBe(tokenColor(color));
    expect(px(getComputedStyle(path).strokeWidth)).toBe(2);
    expect(path.getAttribute('stroke-linecap')).toBe('round');
  });

  it('thickens the line to 3 px when active', async () => {
    const { container } = drawEdge({ state: 'succeeded', items: 1, active: true });
    const path = await findPath(container, 'path[data-edge-state="succeeded"]');
    expect(px(getComputedStyle(path).strokeWidth)).toBe(3);
  });

  it('puts a 7 px arrowhead at the target', async () => {
    const { container } = drawEdge({ state: 'succeeded', items: 1 });
    const arrow = await findPath(container, '[data-edge-arrow]');
    expect(Math.round(arrow.getBBox().width)).toBe(7);
  });
});

describe('the connection label', () => {
  it.each([
    [1, '1 item'],
    [3, '3 items'],
    [0, '0 items'],
  ])('writes %i as "%s"', (items, text) => {
    const { container } = stage('light', <EdgeLabel items={items} state="succeeded" />);
    expect(container).toHaveTextContent(text);
  });

  it('writes "unknown" for an uncertain connection and appends " · pinned" for pinned data', () => {
    const { container } = stage('light', (
      <>
        <EdgeLabel state="uncertain" />
        <EdgeLabel items={2} state="succeeded" pinned />
      </>
    ));
    expect(container).toHaveTextContent('unknown');
    expect(container).toHaveTextContent('2 items · pinned');
  });

  it('is a pill of at least 20 px, in the state color, with a 12 px text', () => {
    const { container } = stage('light', <EdgeLabel items={3} state="failed" />);
    const label = must(container.querySelector<HTMLElement>('[data-edge-label]'));
    expect(rect(label).height).toBeGreaterThanOrEqual(20);
    expect(getComputedStyle(label).color).toBe(tokenColor('--orcon-status-danger-fg'));
    expect(px(getComputedStyle(label).fontSize)).toBe(12);
  });
});

