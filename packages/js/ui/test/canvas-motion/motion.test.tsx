// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { cleanup, render, waitFor } from '@testing-library/react';
import { page } from 'vitest/browser';
import { afterEach, beforeAll, describe, expect, it } from 'vitest';

import '../../src/styles/index.css';
import { exampleDraft, exampleGraph } from '../../src/canvas/fixtures';
import { WorkflowCanvas } from '../../src/canvas/WorkflowCanvas';
import { Providers } from '../../src/providers/Providers';

beforeAll(async () => {
  await page.viewport(1440, 900);
});

afterEach(cleanup);

const mount = (graph: typeof exampleGraph) =>
  render(
    <Providers theme="light" density="comfortable" locale="en">
      <div className="h-screen">
        <WorkflowCanvas defaultGraph={graph} readOnly />
      </div>
    </Providers>,
  );

/** The element an animation runs on, or the element that holds it for a pseudo-element. */
const target = (animation: Animation) => (animation.effect as KeyframeEffect | null)?.target as Element | null;

describe('motion on the canvas', () => {
  it('has no animation at all when nothing is running', async () => {
    expect(window.matchMedia('(prefers-reduced-motion: reduce)').matches).toBe(false);
    const { container } = mount({ ...exampleDraft, nodes: exampleGraph.nodes.map((node) => ({ ...node, status: node.status === 'RUNNING' ? 'SUCCEEDED' : node.status })), edges: exampleGraph.edges.map((edge) => ({ ...edge, state: edge.state === 'running' ? 'succeeded' : edge.state })) });
    await waitFor(() => expect(container.querySelector('.react-flow__node')).not.toBeNull());
    await new Promise((resolve) => setTimeout(resolve, 200));
    const loops = document.getAnimations().filter((animation) => animation.effect?.getTiming().iterations === Infinity);
    expect(loops).toEqual([]);
  });

  it('animates only the running node and the running connection', async () => {
    const { container } = mount(exampleGraph);
    await waitFor(() => expect(container.querySelector('[data-run-ring]')).not.toBeNull());
    const loops = document.getAnimations().filter((animation) => animation.effect?.getTiming().iterations === Infinity);
    expect(loops.length).toBeGreaterThan(0);
    for (const animation of loops) {
      const element = target(animation);
      const running = element?.closest('[data-run-ring], [data-edge-state="running"], [data-node-badge="RUNNING"], [data-node-chip="RUNNING"]');
      expect(running, `${element?.tagName} ${String((element as HTMLElement | null)?.className)}`).not.toBeNull();
    }
  });
});
