// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { FlowDocument, FlowNode } from './flow-document';
import { typeDefOf } from './node-types';

/**
 * The node types the palette puts under "Suggested" after a given step (SPEC-019 R-UX-EDT-04). The table is
 * static: the types that usually follow another type, learned from the flows of the organization, are not
 * available yet. Entries are palette slugs (`model/node-types.ts`); the order is the order shown. A slug the
 * palette does not offer yet (the AI agent) is skipped by the palette, never invented.
 */
export const NEXT_STEP_TABLE: Readonly<Record<string, readonly string[]>> = {
  webhook: ['if', 'set', 'http'],
  manual: ['set', 'http', 'if'],
  http: ['if', 'set', 'end'],
  set: ['http', 'if', 'end'],
  if: ['http', 'set', 'end'],
  merge: ['set', 'http', 'end'],
  loop: ['http', 'set', 'merge'],
  call: ['if', 'set', 'end'],
};

/** The connectors whose trigger is a message that arrives: after it the suggestion is AI, If and a reply by the same connector. */
export const MESSAGING_TYPE = /^orcon\.(whatsapp|slack|telegram|messenger|instagram|email)\./;

export interface NextStepSuggestions {
  /** Palette slugs to show first. Empty when nothing is known about the step before. */
  readonly slugs: readonly string[];
  /** The step the suggestions follow, for the heading ("Suggested after Webhook"). */
  readonly after: FlowNode | undefined;
  /** The step before is a message that arrives from a messaging connector: the reply by the same connector is offered too. */
  readonly replyWithSameConnector: boolean;
}

/** Whether a node is a message that arrives from a messaging connector. */
export function isInboundMessage(node: Pick<FlowNode, 'type'>): boolean {
  return MESSAGING_TYPE.test(node.type);
}

/**
 * What to suggest after `previous`. After an inbound message: AI, If and a reply by the same connector;
 * after any other known step the row of the static table; after an unknown one, nothing.
 */
export function nextStepSuggestions(previous: FlowNode | undefined): NextStepSuggestions {
  if (previous === undefined) return { slugs: [], after: undefined, replyWithSameConnector: false };
  if (isInboundMessage(previous)) return { slugs: ['ai', 'if'], after: previous, replyWithSameConnector: true };
  const slug = typeDefOf(previous)?.slug;
  const row = slug === undefined ? undefined : NEXT_STEP_TABLE[slug];
  return { slugs: row ?? [], after: previous, replyWithSameConnector: false };
}

/** What the palette offers when a search finds nothing: an HTTP call (it can reach any service) and the AI builder (coming soon). */
export type NoMatchAction = 'http-request' | 'ask-ai';

export function noMatchActions(matches: number): readonly NoMatchAction[] {
  return matches > 0 ? [] : ['http-request', 'ask-ai'];
}

/**
 * Whether the flow starts from a chat or a messaging trigger, so the test chat makes sense for it (SPEC-019
 * R-UX-EDT-05): a start node of a messaging connector, or of the chat trigger. A webhook or a manual start does not count.
 */
export function startsFromChat(document: Pick<FlowDocument, 'nodes'>): boolean {
  return document.nodes.some((node) => node.element === 'startEvent' && (isInboundMessage(node) || node.type.startsWith('orcon.chat.')));
}
