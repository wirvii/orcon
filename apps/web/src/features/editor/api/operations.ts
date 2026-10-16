// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { z } from 'zod';

import { apiCall } from '../../../app/api/http';
import { ContractError } from '../../../app/errors/contract-error';
import { flowDocumentSchema, type FlowDocument } from '../model/flow-document';

/** The draft of a flow as the API sends it (SPEC-005 `FlowDraft`). The revision travels as the ETag. */
const draftSchema = z.object({
  flowId: z.uuid(),
  revision: z.number().int().min(0),
  document: flowDocumentSchema,
  updatedAt: z.string(),
});

/** The document of a draft and the entity tag it was read at (`If-Match` on the next save). */
export interface DraftAnswer {
  document: FlowDocument;
  etag: string;
}

/** Reads the editable draft of a flow (`getDraft`). */
export async function getDraft(flowId: string): Promise<DraftAnswer> {
  const source = 'response of GET /flows/{flowId}/draft';
  const { value, etag } = await apiCall({
    method: 'GET',
    path: `/flows/${encodeURIComponent(flowId)}/draft`,
    schema: draftSchema,
    source,
  });
  if (etag === undefined) throw new ContractError(source, []);
  return { document: value.document, etag };
}

/**
 * Replaces the whole document of the draft (`replaceDraft`), only when the draft is still at
 * `ifMatch`: the server answers 412 otherwise and nothing is overwritten. Returns the new entity tag.
 */
export async function replaceDraft(flowId: string, document: FlowDocument, ifMatch: string): Promise<string> {
  const source = 'response of PUT /flows/{flowId}/draft';
  const { etag } = await apiCall({
    method: 'PUT',
    path: `/flows/${encodeURIComponent(flowId)}/draft`,
    body: { document },
    ifMatch,
    schema: draftSchema,
    source,
  });
  if (etag === undefined) throw new ContractError(source, []);
  return etag;
}

/**
 * The document of a published version (`getFlowVersion`). A version never changes, so the answer for a number is
 * always the same; the editor compares the draft with it to count the changes the next publication would carry.
 */
export async function getVersionDocument(flowId: string, version: number): Promise<FlowDocument> {
  const { value } = await apiCall({
    method: 'GET',
    path: `/flows/${encodeURIComponent(flowId)}/versions/${String(version)}`,
    schema: z.looseObject({ version: z.number().int().min(1), document: flowDocumentSchema }),
    source: 'response of GET /flows/{flowId}/versions/{version}',
  });
  return value.document;
}
