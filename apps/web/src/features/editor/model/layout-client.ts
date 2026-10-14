// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { z } from 'zod';

import type { Point } from './flow-document';
import type { LayoutInput } from './layout';

const outputSchema = z.array(z.tuple([z.string(), z.object({ x: z.number(), y: z.number() })]));

async function inThisThread(input: LayoutInput): Promise<Map<string, Point>> {
  // The layout library stays out of the editor's first chunk: it loads only when somebody tidies up.
  const { layoutGraph } = await import('./layout');
  return new Map(layoutGraph(input));
}

/**
 * Runs the layout in a web worker so a large flow does not freeze the page (SPEC-014 R-ED-CAN-03).
 * Where there is no worker (tests, a locked-down browser) or the worker fails, it runs in this
 * thread instead: the answer is the same.
 */
export function computeLayout(input: LayoutInput): Promise<Map<string, Point>> {
  if (typeof Worker === 'undefined') return inThisThread(input);
  return new Promise((resolve) => {
    let worker: Worker;
    try {
      worker = new Worker(new URL('./layout.worker.ts', import.meta.url), { type: 'module' });
    } catch {
      resolve(inThisThread(input));
      return;
    }
    worker.onmessage = (event: MessageEvent<unknown>) => {
      worker.terminate();
      const parsed = outputSchema.safeParse(event.data);
      resolve(parsed.success ? new Map(parsed.data) : inThisThread(input));
    };
    worker.onerror = () => {
      worker.terminate();
      resolve(inThisThread(input));
    };
    worker.postMessage(input);
  });
}
