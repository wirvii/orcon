// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { FlowDocument } from '../model/flow-document';

/** A draft as the editor receives it: the document and the revision it was read at. */
export interface LoadedDraft {
  document: FlowDocument;
  /** Opaque to the editor; saving sends it back as `If-Match` (SPEC-014 R-ED-DRAFT-01). */
  revision: string;
}

export type SaveResult = { kind: 'saved'; revision: string } | { kind: 'conflict' };

/**
 * Where the editor reads and writes the draft of a flow. This is the whole surface the editor needs
 * from storage, so connecting it to the server later means writing one more implementation (the
 * SDK operations `getDraft` and the JSON Patch save of SPEC-005) and choosing it in `queries.ts`;
 * the store, the screens and the autosave do not change. `load` rejects with a problem object
 * (status 404 for a flow that does not exist) and `save` rejects when the request fails.
 */
export interface DraftSource {
  load: (flowId: string) => Promise<LoadedDraft>;
  /** Writes the document if the draft is still at `ifMatch`; answers `conflict` when it is not. */
  save: (flowId: string, document: FlowDocument, ifMatch: string) => Promise<SaveResult>;
}
