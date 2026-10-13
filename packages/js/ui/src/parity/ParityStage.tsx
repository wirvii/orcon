// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Graph } from '../canvas/model';
import { Toaster } from '../components/toast/Toast';
import { EditorFrame } from '../screens/EditorFrame';

import { ParityEnv, type Driver } from './env';
import { parityScripts } from './scripts';

/** The editor as it is when a parity row starts, and the latest graph it reported. One stage is on screen at a time. */
export const parityState: { graph: Graph | undefined } = { graph: undefined };

export function ParityStage({ row }: { row: string }) {
  const script = parityScripts[row];
  if (!script) throw new Error(`No parity script for ${row}`);
  parityState.graph = script.frame.graph;
  return (
    <div className="h-screen p-4">
      <EditorFrame {...script.frame} onGraphChange={(graph) => (parityState.graph = graph)} />
      <Toaster />
    </div>
  );
}

/** An environment for a script on the stage that is on screen. */
export function parityEnv(driver: Driver, root: HTMLElement, row: string): ParityEnv {
  return new ParityEnv(driver, root, () => parityState.graph ?? parityScripts[row]?.frame.graph ?? { nodes: [], edges: [] });
}
