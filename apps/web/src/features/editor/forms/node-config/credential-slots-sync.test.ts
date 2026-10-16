// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { expect, it } from 'vitest';

import { MODEL_CONNECTOR, SERVICES, WHATSAPP_CONNECTOR } from '../../../../app/credentials/model/services';
import { credentialSlotsOf } from '../../model/credential-slots';
import { exampleFlowDocument } from '../../model/example-flow';

// The editor's model cannot import the credential screen's services, so it spells the connector references itself.
// This keeps the two spellings from drifting apart: every type a slot accepts is one the dialog can create.
it('every connector a credential slot accepts is a service the create dialog offers', () => {
  const creatable = new Set(SERVICES.map((service) => service.connector));
  const steps = [...exampleFlowDocument().nodes, ...['orcon.whatsapp.whatsappTrigger@1', 'orcon.whatsapp.whatsapp@1', 'orcon.ai.llm@1', 'orcon.ai.agent@1'].map((type) => ({ id: type, name: type, element: 'serviceTask', type, parameters: {} }))];
  const accepted = new Set(steps.flatMap((node) => credentialSlotsOf(node).flatMap((slot) => slot.types)));
  expect(accepted).toEqual(new Set(['orcon.http@1', WHATSAPP_CONNECTOR, MODEL_CONNECTOR]));
  for (const type of accepted) expect(creatable.has(type)).toBe(true);
});
