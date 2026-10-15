// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';

import { exampleFlowDocument } from '../../model/example-flow';
import { findNode, type FlowDocument } from '../../model/flow-document';

import { renderWithStore } from './harness';
import { NodeConfigForm } from './NodeConfigForm';

const withNode = (node: Record<string, unknown>): FlowDocument => {
  const flow = exampleFlowDocument();
  flow.nodes.push(node as FlowDocument['nodes'][number]);
  return flow;
};

describe('the WhatsApp node', () => {
  it('asks for the recipient and the message object of a send, and for the message ID only when it marks a message as read', async () => {
    const send = renderWithStore(<NodeConfigForm nodeId="wa" />, withNode({ id: 'wa', name: 'Reply', element: 'serviceTask', type: 'orcon.whatsapp.whatsapp@1', parameters: { operation: 'message.send' } }));
    expect(await screen.findByText('Recipient')).toBeVisible();
    expect(screen.getByText('Text')).toBeVisible();
    expect(screen.queryByText('Message ID')).toBeNull();
    send.unmount();

    renderWithStore(<NodeConfigForm nodeId="wa" />, withNode({ id: 'wa', name: 'Read', element: 'serviceTask', type: 'orcon.whatsapp.whatsapp@1', parameters: { operation: 'message.update' } }));
    expect(await screen.findByText('Message ID')).toBeVisible();
    expect(screen.queryByText('Recipient')).toBeNull();
  });

  it('lists the WhatsApp account credential as the one the node needs', async () => {
    renderWithStore(<NodeConfigForm nodeId="wa" />, withNode({ id: 'wa', name: 'Reply', element: 'serviceTask', type: 'orcon.whatsapp.whatsapp@1', parameters: { operation: 'message.send' } }));
    expect(await screen.findByRole('combobox', { name: /WhatsApp account/ })).toBeInTheDocument();
  });
});

describe('the language model node', () => {
  it('writes the provider and the model name as one model reference', async () => {
    const { store } = renderWithStore(
      <NodeConfigForm nodeId="llm" />,
      withNode({ id: 'llm', name: 'Summarize', element: 'serviceTask', type: 'orcon.ai.llm@1', parameters: { model: { provider: 'provider-a', name: 'model-x' } } }),
    );
    const name = await screen.findByRole('textbox', { name: 'Model name' });
    expect(name).toHaveValue('model-x');
    expect(screen.getByRole('textbox', { name: 'Provider' })).toHaveValue('provider-a');
    await userEvent.clear(name);
    await userEvent.type(name, 'model-y');
    const model = findNode(store.getState().document, 'llm')?.parameters['model'];
    expect(model).toEqual({ provider: 'provider-a', name: 'model-y' });
  });

  it('asks for the schema only when the answer must follow one', async () => {
    renderWithStore(
      <NodeConfigForm nodeId="llm" />,
      withNode({ id: 'llm', name: 'Extract', element: 'serviceTask', type: 'orcon.ai.llm@1', parameters: { model: { provider: 'p', name: 'm' }, responseFormat: 'TEXT' } }),
    );
    expect(await screen.findByText('System prompt')).toBeVisible();
    expect(screen.queryByText('Schema')).toBeNull();
  });
});
