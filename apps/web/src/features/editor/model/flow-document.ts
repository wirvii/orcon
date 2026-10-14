// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { z } from 'zod';

/**
 * The part of the `orcon/flow@1` document (docs/reference/orcon-flow-1.md, ADR-0007) the editor
 * reads and writes. Every object is open (`looseObject`): members this editor does not know are
 * kept as they are, so a document saved from here never loses what another client or a later
 * release wrote. Until the generated contract (`@orcon/api-contract`) carries the draft schema
 * these schemas are the boundary of the editor's own copy of the document.
 */

export const FLOW_FORMAT = 'orcon/flow@1';

/** Node and edge ids (orcon-flow-1.md "Patterns and limits"). */
export const ID_PATTERN = /^[A-Za-z0-9_-]{1,64}$/;
const TYPE_PATTERN = /^[a-z][a-z0-9_-]*(\.[a-z][a-z0-9_-]*)+@[1-9][0-9]*$/;

export const MAX_NOTE_LENGTH = 10_000;
export const STICKY_COLORS = [1, 2, 3, 4, 5, 6] as const;

const pointSchema = z.object({ x: z.number(), y: z.number() });

export const stickyNoteSchema = z.object({
  id: z.string().regex(ID_PATTERN),
  x: z.number(),
  y: z.number(),
  w: z.number().positive(),
  h: z.number().positive(),
  color: z.number().int().min(1).max(6),
  title: z.string().max(200).optional(),
  body: z.string().max(MAX_NOTE_LENGTH),
});

export const flowNodeSchema = z.looseObject({
  id: z.string().regex(ID_PATTERN),
  name: z.string().min(1).max(128),
  element: z.string().min(1),
  type: z.string().regex(TYPE_PATTERN),
  parameters: z.record(z.string(), z.unknown()),
  disabled: z.boolean().optional(),
});

export const flowEdgeSchema = z.looseObject({
  id: z.string().regex(ID_PATTERN),
  from: z.looseObject({ node: z.string(), port: z.string() }),
  to: z.looseObject({ node: z.string() }),
  kind: z.enum(['SEQUENCE', 'BACK']),
  condition: z.string().optional(),
  default: z.boolean().optional(),
});

export const editorSchema = z.looseObject({
  nodes: z.record(z.string(), pointSchema).optional(),
  stickyNotes: z.array(stickyNoteSchema).optional(),
});

export const flowDocumentSchema = z.looseObject({
  format: z.literal(FLOW_FORMAT),
  id: z.string().min(1),
  name: z.string().min(1).max(256),
  description: z.string().optional(),
  settings: z.record(z.string(), z.unknown()),
  variables: z.array(z.unknown()),
  nodes: z.array(flowNodeSchema),
  edges: z.array(flowEdgeSchema),
  editor: editorSchema,
});

export type Point = z.infer<typeof pointSchema>;
export type StickyNote = z.infer<typeof stickyNoteSchema>;
export type FlowNode = z.infer<typeof flowNodeSchema>;
export type FlowEdge = z.infer<typeof flowEdgeSchema>;
export type FlowDocument = z.infer<typeof flowDocumentSchema>;
export type FlowEdgeKind = FlowEdge['kind'];

export type DocumentParse = { readonly ok: true; readonly document: FlowDocument } | { readonly ok: false; readonly message: string };

/** Parses a value as a flow document. The message names the first problem and its JSON Pointer. */
export function parseFlowDocument(value: unknown): DocumentParse {
  const result = flowDocumentSchema.safeParse(value);
  if (result.success) return { ok: true, document: result.data };
  const issue = result.error.issues[0];
  const pointer = issue ? `/${issue.path.map(String).join('/')}` : '/';
  return { ok: false, message: `${pointer}: ${issue?.message ?? 'invalid'}` };
}

/** The node with this id, if the document has it. */
export function findNode(document: FlowDocument, id: string): FlowNode | undefined {
  return document.nodes.find((node) => node.id === id);
}

/** Creates the editor members a write needs (`editor.nodes`, `editor.stickyNotes`) and returns them. */
export function editorBlock(document: FlowDocument): { nodes: Record<string, Point>; stickyNotes: StickyNote[] } {
  document.editor.nodes ??= {};
  document.editor.stickyNotes ??= [];
  return { nodes: document.editor.nodes, stickyNotes: document.editor.stickyNotes };
}

/**
 * A deep copy of plain data that shares nothing with the original. Unlike `structuredClone` it
 * also copies the proxies Immer hands to an edit while the store applies it.
 */
export function deepCopy<T>(value: T): T {
  if (Array.isArray(value)) return value.map((item: unknown) => deepCopy(item)) as T;
  if (typeof value === 'object' && value !== null) {
    return Object.fromEntries(Object.entries(value).map(([key, member]) => [key, deepCopy(member)])) as T;
  }
  return value;
}

export function cloneDocument(document: FlowDocument): FlowDocument {
  return deepCopy(document);
}

/** Every id a new node, edge or note must not reuse. */
export function takenIds(document: FlowDocument): Set<string> {
  return new Set([
    ...document.nodes.map((node) => node.id),
    ...document.edges.map((edge) => edge.id),
    ...(document.editor.stickyNotes ?? []).map((note) => note.id),
  ]);
}

/** The first `${stem}-${n}` (n from 1) that is not taken. */
export function nextId(stem: string, taken: ReadonlySet<string>): string {
  let index = 1;
  const clean = stem.slice(0, 56);
  while (taken.has(`${clean}-${String(index)}`)) index += 1;
  return `${clean}-${String(index)}`;
}
