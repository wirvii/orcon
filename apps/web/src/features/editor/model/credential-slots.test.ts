// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from 'vitest';

import { credentialSlotsOf } from './credential-slots';
import { exampleFlowDocument } from './example-flow';

const step = (type: string) => ({ type, element: 'serviceTask', parameters: {} });

describe('credential slots', () => {
  it('the WhatsApp trigger and the WhatsApp step take the WhatsApp credential from `credential`', () => {
    for (const type of ['orcon.whatsapp.whatsappTrigger@1', 'orcon.whatsapp.whatsapp@1']) {
      expect(credentialSlotsOf(step(type))).toEqual([expect.objectContaining({ types: ['orcon.whatsapp@1'], parameter: ['credential'], required: true })]);
    }
  });

  it('the AI model and the AI agent take the model credential from inside the model reference', () => {
    for (const type of ['orcon.ai.llm@1', 'orcon.ai.agent@1']) {
      expect(credentialSlotsOf(step(type))).toEqual([expect.objectContaining({ types: ['orcon.ai.llm@1'], parameter: ['model', 'credential'], required: true })]);
    }
  });

  it('the HTTP step keeps the slot of its manifest and a step without credentials has none', () => {
    const nodes = exampleFlowDocument().nodes;
    const http = nodes.find((node) => node.id === 'slack');
    expect(http && credentialSlotsOf(http)).toEqual([expect.objectContaining({ slot: 'auth', types: ['orcon.http@1'], parameter: ['credential'] })]);
    expect(credentialSlotsOf(step('orcon.set@1'))).toEqual([]);
  });
});
