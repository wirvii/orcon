// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { useCallback, useEffect, useRef, useState } from 'react';

import type { EdgeExecutionState, ExecutionOverlay, NodeExecutionState } from '../../canvas/WorkflowCanvas';
import type { Graph, GraphNode } from '../../canvas/model';
import { orderItems } from '../fixtures';
import type { DataItem } from '../model';

/** The items a node produces in the sample runs: the same three orders, split over the outputs of a node with several. */
export function sampleOutputs(node: GraphNode): Record<string, DataItem[]> {
  if (node.outputs.length <= 1) return { [node.outputs[0]?.id ?? 'main']: [...orderItems] };
  const per = Math.max(1, Math.floor(orderItems.length / node.outputs.length));
  return Object.fromEntries(node.outputs.map((port, index) => [port.id, index === node.outputs.length - 1 ? orderItems.slice(index * per) : orderItems.slice(index * per, (index + 1) * per)]));
}

/** What a node holds after a run, by output id. */
export type NodeOutputs = Record<string, Record<string, DataItem[]>>;

export interface RunSimulation {
  overlay: ExecutionOverlay;
  outputs: NodeOutputs;
  /** Which node is running now. */
  running: ReadonlySet<string>;
  runStep: (nodeId: string) => void;
  executeWorkflow: (triggerId: string) => void;
}

/** Order of the nodes reachable from a trigger, following connections forward (loops excluded). */
function executionOrder(graph: Graph, startId: string): string[] {
  const seen = new Set<string>([startId]);
  const order = [startId];
  for (let index = 0; index < order.length; index += 1) {
    for (const edge of graph.edges) {
      if (edge.source === order[index] && !edge.loopBack && !seen.has(edge.target)) {
        seen.add(edge.target);
        order.push(edge.target);
      }
    }
  }
  return order;
}

/**
 * Runs steps of the sample flow with timers, for the editor template and the stories: no request
 * is made. A step turns RUNNING, then SUCCEEDED; its connections show the item counts. A pinned node
 * keeps its data and succeeds at once; a disabled node is skipped.
 */
export function useRunSimulation(getGraph: () => Graph, initial: ExecutionOverlay | undefined, delay = 700, initialOutputs: NodeOutputs = {}): RunSimulation {
  const [nodes, setNodes] = useState<Record<string, NodeExecutionState>>(() => ({ ...initial?.nodes }));
  const [edges, setEdges] = useState<Record<string, EdgeExecutionState>>(() => ({ ...initial?.edges }));
  const [outputs, setOutputs] = useState<NodeOutputs>(initialOutputs);
  const [running, setRunning] = useState<ReadonlySet<string>>(new Set());
  const timers = useRef(new Set<ReturnType<typeof setTimeout>>());
  const outputsRef = useRef(outputs);
  outputsRef.current = outputs;

  useEffect(
    () => () => {
      for (const timer of timers.current) clearTimeout(timer);
    },
    [],
  );

  const later = useCallback((fn: () => void, ms: number) => {
    const timer = setTimeout(() => {
      timers.current.delete(timer);
      fn();
    }, ms);
    timers.current.add(timer);
  }, []);

  const finish = useCallback((nodeId: string) => {
    const graph = getGraph();
    const node = graph.nodes.find((candidate) => candidate.id === nodeId);
    if (!node) return;
    const produced = outputsRef.current[nodeId] ?? sampleOutputs(node);
    setOutputs((current) => ({ ...current, [nodeId]: produced }));
    setNodes((current) => ({ ...current, [nodeId]: { status: 'SUCCEEDED' } }));
    setEdges((current) => {
      const next = { ...current };
      for (const edge of graph.edges) {
        if (edge.target === nodeId) next[edge.id] = { state: 'succeeded', items: next[edge.id]?.items ?? produced[Object.keys(produced)[0] ?? 'main']?.length ?? 0 };
        if (edge.source === nodeId) next[edge.id] = { state: 'succeeded', items: produced[edge.sourceOutput]?.length ?? 0 };
      }
      return next;
    });
    setRunning((current) => {
      const next = new Set(current);
      next.delete(nodeId);
      return next;
    });
  }, [getGraph]);

  const start = useCallback(
    (nodeId: string, then?: () => void) => {
      const node = getGraph().nodes.find((candidate) => candidate.id === nodeId);
      if (!node) return;
      if (node.disabled) {
        setEdges((current) => {
          const next = { ...current };
          for (const edge of getGraph().edges) if (edge.source === nodeId || edge.target === nodeId) next[edge.id] = { state: 'not-run', items: 0 };
          return next;
        });
        then?.();
        return;
      }
      if (node.pinned) {
        finish(nodeId);
        then?.();
        return;
      }
      setNodes((current) => ({ ...current, [nodeId]: { status: 'RUNNING' } }));
      setRunning((current) => new Set(current).add(nodeId));
      setEdges((current) => {
        const next = { ...current };
        for (const edge of getGraph().edges) if (edge.target === nodeId) next[edge.id] = { state: 'running', items: next[edge.id]?.items ?? orderItems.length };
        return next;
      });
      later(() => {
        finish(nodeId);
        then?.();
      }, delay);
    },
    [delay, finish, getGraph, later],
  );

  const runStep = useCallback((nodeId: string) => start(nodeId), [start]);

  const executeWorkflow = useCallback(
    (triggerId: string) => {
      const order = executionOrder(getGraph(), triggerId);
      // A fresh run: every node and connection of this run starts from nothing.
      setNodes({});
      setEdges({});
      const step = (index: number) => {
        const id = order[index];
        if (!id) return;
        start(id, () => step(index + 1));
      };
      step(0);
    },
    [getGraph, start],
  );

  return { overlay: { nodes, edges }, outputs, running, runStep, executeWorkflow };
}
