// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Generates perf/fixtures/reference-flow-500.json, the flow every editor performance measurement uses
// (orcon-web PF-4): 500 nodes (420 action nodes including the trigger, 80 control nodes), 560
// connections of which 12 loop back, laid out on an 8 px grid. The seed is fixed, so the output is
// identical on every run; the file is committed and `--check` fails when it is out of date.
//
//   node perf/generate-reference-flow.mjs [--check]

import { readFileSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const SEED = 20261013;
const NODES = 500;
const CONTROL_NODES = 80;
const EDGES = 560;
const LOOP_BACKS = 12;
const COLUMNS = 25;
const COLUMN_PITCH = 240;
const ROW_PITCH = 160;

/** A small seeded generator (mulberry32): the same numbers on every run and every machine. */
function random(seed) {
  let state = seed >>> 0;
  return () => {
    state = (state + 0x6d2b79f5) >>> 0;
    let t = state;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

const rand = random(SEED);
const pick = (list) => list[Math.floor(rand() * list.length)];
const id = (prefix, index) => `${prefix}${String(index + 1).padStart(3, '0')}`;
const snap = (value) => Math.round(value / 8) * 8;

const icons = ['globe', 'database', 'mail', 'message', 'file', 'code'];
const verbs = ['Fetch', 'Send', 'Update', 'Create', 'Check', 'Sync', 'Log', 'Parse'];
const things = ['order', 'invoice', 'customer', 'ticket', 'report', 'contact', 'payment', 'record'];

// Control nodes are spread evenly: every sixth node, 80 in all, alternating if and merge.
const controlAt = new Set();
for (let slot = 0; controlAt.size < CONTROL_NODES; slot += 1) {
  const index = 1 + Math.floor((slot * (NODES - 2)) / CONTROL_NODES);
  controlAt.add(index);
}

const nodes = Array.from({ length: NODES }, (_, index) => {
  const column = index % COLUMNS;
  const row = Math.floor(index / COLUMNS);
  const position = { x: snap(column * COLUMN_PITCH), y: snap(row * ROW_PITCH) };
  if (index === 0) {
    return { id: id('n', index), kind: 'trigger', name: 'Start', subtitle: 'Webhook', icon: 'webhook', inputs: [], outputs: ['main'], position };
  }
  if (controlAt.has(index)) {
    const control = [...controlAt].sort((a, b) => a - b).indexOf(index) % 2 === 0 ? 'if' : 'merge';
    return control === 'if'
      ? { id: id('n', index), kind: 'control', logic: 'if', name: `If ${pick(things)} is ready`, subtitle: 'Branch', inputs: ['main'], outputs: ['true', 'false'], position }
      : { id: id('n', index), kind: 'control', logic: 'merge', name: 'Merge branches', subtitle: 'Wait for both', inputs: ['a', 'b'], outputs: ['main'], position };
  }
  return { id: id('n', index), kind: 'action', name: `${pick(verbs)} ${pick(things)}`, subtitle: 'Do something', icon: pick(icons), inputs: ['main'], outputs: ['main'], position };
});

const edges = [];
const usedOutputs = new Map();
const usedInputs = new Map();
const nextOutput = (node) => {
  const used = usedOutputs.get(node.id) ?? 0;
  usedOutputs.set(node.id, used + 1);
  return node.outputs[used % node.outputs.length];
};
const nextInput = (node) => {
  const used = usedInputs.get(node.id) ?? 0;
  usedInputs.set(node.id, used + 1);
  return node.inputs[Math.min(used, node.inputs.length - 1)];
};
const connect = (source, target, loopBack = false) => {
  edges.push({
    id: id('e', edges.length),
    source: source.id,
    sourceOutput: nextOutput(source),
    target: target.id,
    targetInput: nextInput(target),
    ...(loopBack ? { loopBack: true } : {}),
  });
};

// A connection into every node but the trigger: from its left neighbour, or from the node above at the start of a row.
for (let index = 1; index < NODES; index += 1) {
  const parent = index % COLUMNS === 0 ? index - COLUMNS : index - 1;
  connect(nodes[parent], nodes[index]);
}
// Extra forward connections that skip ahead a few nodes.
const forwardExtras = EDGES - LOOP_BACKS - edges.length;
for (let extra = 0; extra < forwardExtras; extra += 1) {
  const from = 1 + Math.floor(rand() * (NODES - 8));
  connect(nodes[from], nodes[from + 2 + Math.floor(rand() * 5)]);
}
// Connections that go back to an earlier node: the loops.
for (let loop = 0; loop < LOOP_BACKS; loop += 1) {
  const from = 60 + Math.floor(rand() * (NODES - 60));
  connect(nodes[from], nodes[Math.max(1, from - 3 - Math.floor(rand() * 30))], true);
}

// One node or connection per line keeps the committed file small and its diffs readable.
const lines = (items) => items.map((item) => `  ${JSON.stringify(item)}`).join(',\n');
const text = `{\n "format": "orcon/perf-reference-flow@1",\n "seed": ${SEED},\n "nodes": [\n${lines(nodes)}\n ],\n "edges": [\n${lines(edges)}\n ]\n}\n`;

const target = join(dirname(fileURLToPath(import.meta.url)), 'fixtures', 'reference-flow-500.json');
if (process.argv.includes('--check')) {
  if (readFileSync(target, 'utf8') !== text) {
    console.error('reference-flow-500.json is out of date: run `node perf/generate-reference-flow.mjs`');
    process.exit(1);
  }
  console.log('reference-flow-500.json is up to date');
} else {
  writeFileSync(target, text);
  const loops = edges.filter((edge) => edge.loopBack).length;
  const control = nodes.filter((node) => node.kind === 'control').length;
  console.log(`wrote ${target}: ${nodes.length} nodes (${nodes.length - control} action, ${control} control), ${edges.length} connections (${loops} loop back)`);
}
