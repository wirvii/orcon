// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { z } from 'zod';

import { apiCall, apiRequest } from '../../../app/api/http';
import { ContractError } from '../../../app/errors/contract-error';

/** The state of a flow the editor needs to publish and activate it (SPEC-005 `Flow`; the contract may add fields). */
export const flowStateSchema = z.looseObject({
  id: z.uuid(),
  activationState: z.enum(['ACTIVE', 'INACTIVE']),
  activeVersion: z.number().int().min(1).nullish(),
  hasUnpublishedChanges: z.boolean(),
  draftRevision: z.number().int().min(0),
});

/** A published version, as `publishFlow` answers it. */
export const flowVersionSchema = z.looseObject({
  flowId: z.uuid(),
  version: z.number().int().min(1),
  contentHash: z.string(),
  publishedAt: z.string(),
  note: z.string().nullish(),
});

export type FlowState = z.infer<typeof flowStateSchema>;
export type FlowVersion = z.infer<typeof flowVersionSchema>;

/** The state of a flow and the entity tag it was read at (`If-Match` when it is activated or deactivated). */
export interface FlowAnswer {
  flow: FlowState;
  etag: string;
}

/** Reads the state of a flow (`getFlow`). */
export async function getFlow(flowId: string): Promise<FlowAnswer> {
  const source = 'response of GET /flows/{flowId}';
  const { value, etag } = await apiCall({
    method: 'GET',
    path: `/flows/${encodeURIComponent(flowId)}`,
    schema: flowStateSchema,
    source,
  });
  if (etag === undefined) throw new ContractError(source, []);
  return { flow: value, etag };
}

/**
 * Publishes the current draft as a new immutable version (`publishFlow`). The note is optional and is
 * stored with the version; an empty note is not sent.
 */
export function publishFlow(flowId: string, note: string | undefined): Promise<FlowVersion> {
  return apiRequest({
    method: 'POST',
    path: `/flows/${encodeURIComponent(flowId)}/publish`,
    body: note === undefined || note === '' ? {} : { note },
    schema: flowVersionSchema,
    source: 'response of POST /flows/{flowId}/publish',
  });
}

/**
 * Switches the automatic triggers of a flow on or off (`activateFlow`, `deactivateFlow`). The flow must
 * have been published first; a stale entity tag is refused with 412.
 */
export async function setFlowActivation(flowId: string, active: boolean, ifMatch: string): Promise<FlowAnswer> {
  const action = active ? 'activate' : 'deactivate';
  const source = `response of POST /flows/{flowId}/${action}`;
  const { value, etag } = await apiCall({
    method: 'POST',
    path: `/flows/${encodeURIComponent(flowId)}/${action}`,
    ifMatch,
    schema: flowStateSchema,
    source,
  });
  if (etag === undefined) throw new ContractError(source, []);
  return { flow: value, etag };
}
