// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { FlowNode } from './flow-document';
import { manifestOf, type CredentialSlot } from './node-manifests';

/**
 * The credential slots of the node types that have no manifest in the editor yet but do take a stored
 * credential. The slot says which credentials fit (their `type` is the connector reference they were created
 * for) and where the node keeps the identifier of the one chosen: the engine reads it from `parameters`, the
 * same place the HTTP step reads its own.
 *
 * - The WhatsApp package serves its send step and its trigger with one credential: the send step reads its
 *   `token`, the trigger its `appSecret` and `verifyToken`.
 * - An AI model or agent step names its credential inside its model reference (`model.credential`), which the
 *   engine reads for the API key of the model.
 */
const WHATSAPP_SLOT: CredentialSlot = { slot: 'whatsapp', types: ['orcon.whatsapp@1'], parameter: ['credential'], required: true };
const MODEL_SLOT: CredentialSlot = { slot: 'model', types: ['orcon.ai.llm@1'], parameter: ['model', 'credential'], required: true };

const BY_TYPE = new Map<string, CredentialSlot>([
  ['orcon.whatsapp.whatsappTrigger@1', WHATSAPP_SLOT],
  ['orcon.whatsapp.whatsapp@1', WHATSAPP_SLOT],
  ['orcon.ai.llm@1', MODEL_SLOT],
  ['orcon.ai.agent@1', MODEL_SLOT],
]);

/** The credential slots of a node: the ones its manifest declares, or the built-in slot of its type. */
export function credentialSlotsOf(node: Pick<FlowNode, 'type' | 'element' | 'parameters'>): readonly CredentialSlot[] {
  const manifest = manifestOf(node);
  if (manifest !== undefined && manifest.credentials.length > 0) return manifest.credentials;
  const own = BY_TYPE.get(node.type);
  return own === undefined ? [] : [own];
}

/** Whether the slot's texts come from the manifest catalog of the node (true) or from the editor's own messages (false). */
export const slotInManifest = (node: Pick<FlowNode, 'type' | 'element' | 'parameters'>, slot: CredentialSlot): boolean => manifestOf(node)?.credentials.includes(slot) ?? false;
