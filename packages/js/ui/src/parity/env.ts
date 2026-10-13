// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { expect, waitFor, within } from 'storybook/test';

import type { Graph } from '../canvas/model';

/** How a script reaches the page: real input in the browser test, synthetic events in Storybook. */
export interface Driver {
  click(element: Element): Promise<void>;
  dblClick(element: Element): Promise<void>;
  hover(element: Element): Promise<void>;
  /** Keys in the user-event syntax: `a`, `{Enter}`, `{Shift>}{ArrowRight}{/Shift}`. */
  keyboard(keys: string): Promise<void>;
  /** Types text into a field (focuses it first). */
  type(element: Element, text: string): Promise<void>;
  /** Presses on `from`, moves to `to` and releases. Not available in Storybook. */
  drag?(from: Element, to: Element, options?: { sourcePosition?: { x: number; y: number }; targetPosition?: { x: number; y: number } }): Promise<void>;
}

export interface Point {
  x: number;
  y: number;
}

/**
 * What a parity script uses. Every call that commits something counts as one step; reaching a
 * target (hover, focus) and typing the content of a field do not. `unit` checks a group of steps
 * against the most the row allows.
 */
export class ParityEnv {
  /** Steps counted in the current unit. */
  steps = 0;
  /** The units that ran, with their step counts, for the report. */
  readonly units: { label: string; steps: number; max: number }[] = [];

  constructor(
    readonly driver: Driver,
    readonly root: HTMLElement,
    private readonly graph: () => Graph,
  ) {}

  get body(): HTMLElement {
    return this.root.ownerDocument.body;
  }

  /** Queries over the whole page: dialogs and menus render outside the story root. */
  get q() {
    return within(this.body);
  }

  getGraph(): Graph {
    return this.graph();
  }

  /** Runs a group of actions that must take at most `max` steps. */
  async unit(label: string, max: number, fn: () => Promise<void>): Promise<void> {
    this.steps = 0;
    await fn();
    this.units.push({ label, steps: this.steps, max });
    expect(this.steps, `${label}: ${this.steps} steps, at most ${max} allowed`).toBeLessThanOrEqual(max);
  }

  /** Reaching a target: moving the pointer or the focus onto it. Not a step. */
  async reach(fn: () => Promise<void> | void): Promise<void> {
    await fn();
  }

  async click(element: Element) {
    this.steps += 1;
    await this.driver.click(element);
  }
  async dblClick(element: Element) {
    this.steps += 1;
    await this.driver.dblClick(element);
  }
  async keys(keys: string) {
    this.steps += 1;
    await this.driver.keyboard(keys);
  }
  async drag(from: Element, to: Element, options?: Parameters<NonNullable<Driver['drag']>>[2]) {
    if (!this.driver.drag) throw new Error('This driver cannot drag. Run the pointer script in the browser test.');
    this.steps += 1;
    await this.driver.drag(from, to, options);
  }
  /** Shift + click: one step. */
  async shiftClick(element: Element) {
    this.steps += 1;
    await this.driver.keyboard('{Shift>}');
    await this.driver.click(element);
    await this.driver.keyboard('{/Shift}');
  }
  /** A wheel gesture on the canvas: one step. */
  async wheel(element: Element, delta: { x?: number; y?: number; ctrl?: boolean }) {
    this.steps += 1;
    element.dispatchEvent(new WheelEvent('wheel', { deltaX: delta.x ?? 0, deltaY: delta.y ?? 0, ctrlKey: delta.ctrl ?? false, bubbles: true, cancelable: true }));
  }

  async hover(element: Element) {
    await this.driver.hover(element);
  }
  async type(element: Element, text: string) {
    await this.driver.type(element, text);
  }
  focus(element: Element) {
    (element as HTMLElement).focus();
  }

  // ---- Finders -------------------------------------------------------------------------------

  /** The React Flow element of a node, found by the start of its accessible name. */
  async node(name: string): Promise<HTMLElement> {
    return this.q.findByRole('group', { name: new RegExp(`^${name}, `) });
  }
  /** The element inside a node's tile. */
  tileOf(node: HTMLElement): HTMLElement {
    return node.querySelector<HTMLElement>('[data-canvas-node]') as HTMLElement;
  }
  async edge(id: string): Promise<HTMLElement> {
    return waitFor(() => {
      const element = this.root.querySelector<HTMLElement>(`.react-flow__edge[data-id="${id}"]`);
      expect(element, `connection ${id}`).not.toBeNull();
      return element as HTMLElement;
    });
  }
  /** The wide transparent line of a connection: the thing a pointer lands on. */
  async edgeLine(id: string): Promise<Element> {
    const edge = await this.edge(id);
    return edge.querySelector('[data-edge-hit]') as Element;
  }
  pane(): HTMLElement {
    return this.root.querySelector<HTMLElement>('.react-flow__pane') as HTMLElement;
  }
  async button(name: string | RegExp): Promise<HTMLElement> {
    return this.q.findByRole('button', { name });
  }
  async option(name: string | RegExp): Promise<HTMLElement> {
    return this.q.findByRole('option', { name });
  }
  nodeElement(id: string): HTMLElement | null {
    return this.root.querySelector<HTMLElement>(`.react-flow__node[data-id="${id}"]`);
  }
  zoom(): number {
    return Number(this.root.querySelector<HTMLElement>('[data-zoom]')?.dataset.zoom ?? '100');
  }
}
