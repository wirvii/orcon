// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { EXAMPLE_FLOW_ID, exampleFlowDocument } from '../model/example-flow';
import { cloneDocument, type FlowDocument } from '../model/flow-document';

import type { DraftSource } from './draft-source';

/** A failure shaped like the API's problem+json (`status`, `title`, `code`), so the app's error screens read it as they read a server error. */
export class DraftNotFound extends Error {
  readonly status = 404;
  readonly title = 'Flow not found';
  readonly code = 'not-found';
  constructor(flowId: string) {
    super(`No draft for flow ${flowId}`);
  }
}

interface Held {
  document: FlowDocument;
  revision: number;
}

/** How a flow that exists comes to be: a function that makes its first draft. */
export type DraftSeed = () => FlowDocument | Promise<FlowDocument>;
export type DraftSeeds = ReadonlyMap<string, DraftSeed>;

/**
 * A draft source that keeps drafts in memory and makes no request. The preview of the editor uses
 * it with the sample flow: edits and saves work, and they last until the page is reloaded.
 */
export function createLocalDraftSource(seeds: DraftSeeds = new Map([[EXAMPLE_FLOW_ID, exampleFlowDocument]])): DraftSource {
  const drafts = new Map<string, Held>();

  const held = async (flowId: string): Promise<Held> => {
    const existing = drafts.get(flowId);
    if (existing) return existing;
    const create = seeds.get(flowId);
    if (!create) throw new DraftNotFound(flowId);
    const created: Held = { document: await create(), revision: 1 };
    drafts.set(flowId, created);
    return created;
  };

  return {
    load: async (flowId) => {
      const current = await held(flowId);
      return { document: cloneDocument(current.document), revision: String(current.revision) };
    },
    save: async (flowId, document, ifMatch) => {
      const current = await held(flowId);
      if (ifMatch !== String(current.revision)) return { kind: 'conflict' };
      current.document = cloneDocument(document);
      current.revision += 1;
      return { kind: 'saved', revision: String(current.revision) };
    },
  };
}
