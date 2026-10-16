// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { act, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { http, HttpResponse } from 'msw';

import { project } from '../../../../../test/product-fixtures';
import { server } from '../../../../../test/screen-harness';
import { problem } from '../../../../../test/session-fixtures';
import { credentialApi, modelCredential, slackWebhook, whatsappCredential } from '../../../../app/credentials/api/fixtures';
import { EXAMPLE_FLOW_ID, exampleFlowDocument, SAMPLE_SUBFLOWS, SLACK_CREDENTIAL_ID } from '../../model/example-flow';
import { findNode, type FlowDocument } from '../../model/flow-document';
import { manifestOf } from '../../model/node-manifests';

import { ConnectorUpdate } from './ConnectorUpdate';
import { FlowTargetField } from './FlowTargetField';
import { renderWithStore } from './harness';
import { NodeConfigForm } from './NodeConfigForm';
import { NodeDocs } from './NodeDocs';
import { NodeSettingsForm } from './NodeSettingsForm';
import { useNode } from './use-node-config';

afterEach(() => {
  vi.restoreAllMocks();
});

const nodeOf = (document: FlowDocument, id: string) => {
  const node = findNode(document, id);
  if (!node) throw new Error(`no node ${id}`);
  return node;
};

function Settings({ id }: { id: string }) {
  const node = useNode(id);
  return node ? <NodeSettingsForm node={node} manifest={manifestOf(node)} /> : null;
}

function Update({ id }: { id: string }) {
  const node = useNode(id);
  return node ? <ConnectorUpdate node={node} /> : null;
}

function Docs({ id }: { id: string }) {
  const node = useNode(id);
  return node ? <NodeDocs node={node} manifest={manifestOf(node)} /> : null;
}

function Call({ id }: { id: string }) {
  const node = useNode(id);
  return node ? <FlowTargetField nodeId={id} value={node['call']} /> : null;
}

const withCall = (call: Record<string, unknown>, extra: Record<string, unknown> = {}): FlowDocument => {
  const flow = exampleFlowDocument();
  flow.nodes.push({ id: 'call', name: 'Ship order', element: 'callActivity', type: 'orcon.callFlow@1', parameters: {}, call, ...extra });
  return flow;
};

describe('credentials', () => {
  it('lists the credentials of the project that fit the step, with the one the node uses chosen', async () => {
    renderWithStore(<NodeConfigForm nodeId="slack" />);
    const select = await screen.findByRole('combobox', { name: /Authentication/ });
    await waitFor(() => {
      expect(select).toHaveTextContent('Slack incoming webhook');
    });
    // Secret values are never shown, and the note says so.
    expect(screen.getByText(/Its value is never shown/)).toBeInTheDocument();
    // Only what fits: the WhatsApp and the model credentials of the same project are not offered to an HTTP step.
    await userEvent.click(select);
    const options = within(await screen.findByRole('listbox')).getAllByRole('option');
    expect(options.map((option) => option.textContent)).toEqual([expect.stringMatching(/No credential/), expect.stringMatching(/^Slack incoming webhook/)]);
  });

  it('keeps the identifier of the chosen credential in the parameters, not its name', async () => {
    const user = userEvent.setup();
    const { store } = renderWithStore(<NodeConfigForm nodeId="slack" />);
    await user.click(await screen.findByRole('combobox', { name: /Authentication/ }));
    await user.click(await screen.findByRole('option', { name: /No credential/ }));
    expect(nodeOf(store.getState().document, 'slack').parameters).not.toHaveProperty('credential');
    await user.click(screen.getByRole('combobox', { name: /Authentication/ }));
    await user.click(await screen.findByRole('option', { name: /^Slack incoming webhook/ }));
    expect(nodeOf(store.getState().document, 'slack').parameters['credential']).toBe(SLACK_CREDENTIAL_ID);
    expect(nodeOf(store.getState().document, 'slack')).not.toHaveProperty('credentials');
  });

  it('says a credential the project does not have is missing and offers a way out', async () => {
    const user = userEvent.setup();
    const { store } = renderWithStore(<NodeConfigForm nodeId="slack" />);
    await screen.findByRole('combobox', { name: /Authentication/ });
    act(() => {
      store.getState().setCredential('slack', ['credential'], '0195f1a0-0000-7000-8000-0000000000ff');
    });
    expect(await screen.findByText('The credential of this step is not in this project')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Choose another' }));
    expect(nodeOf(store.getState().document, 'slack').parameters).not.toHaveProperty('credential');
    await waitFor(() => {
      expect(screen.queryByText('The credential of this step is not in this project')).not.toBeInTheDocument();
    });
  });

  it('says the credentials could not be read, with a way to try again', async () => {
    renderWithStore(<NodeConfigForm nodeId="slack" />);
    server.use(credentialApi.listWith(() => problem(500, 'internal', 'internal')));
    expect(await screen.findByText('The credentials could not be loaded.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Try again' })).toBeInTheDocument();
  });

  it('creates a credential from the node, chooses it and keeps no secret anywhere', async () => {
    const user = userEvent.setup();
    let body: unknown;
    server.use(
      http.post(`/api/v1/projects/${project.id}/credentials`, async ({ request }) => {
        body = await request.json();
        return HttpResponse.json({ ...slackWebhook, id: '0195f1a0-0000-7000-8000-0000000000c9', name: 'Billing key' }, { status: 201, headers: { ETag: '"1-1"' } });
      }),
    );
    const { store, channel, queryClient } = renderWithStore(<NodeConfigForm nodeId="slack" />);
    await user.click(await screen.findByRole('button', { name: 'Create new credential' }));
    const dialog = await screen.findByRole('dialog', { name: 'Create credential' });
    // An HTTP step takes any of the three HTTP services.
    expect(within(dialog).getAllByRole('button', { pressed: true })).toHaveLength(1);
    await user.click(within(dialog).getByRole('button', { name: 'API key in a header' }));
    const secret = within(dialog).getByLabelText(/^Key/);
    expect(secret).toHaveAttribute('type', 'password');
    expect(secret).toHaveValue('');
    await user.type(within(dialog).getByLabelText(/^Credential name/), 'Billing key');
    await user.type(secret, 'hunter2');
    await user.type(within(dialog).getByLabelText(/Send only to these domains/), 'billing.example.com{Enter}');
    await user.click(within(dialog).getByRole('button', { name: 'Create credential' }));
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });
    expect(body).toMatchObject({ name: 'Billing key', type: 'orcon.http@1', allowedHosts: ['billing.example.com'] });
    expect(nodeOf(store.getState().document, 'slack').parameters['credential']).toBe('0195f1a0-0000-7000-8000-0000000000c9');
    expect(channel.current()?.message).toBe('Credential Billing key created and chosen.');
    // The mutation holds no variables after it ends, and no draft carries the secret.
    expect(queryClient.getMutationCache().getAll()).toHaveLength(0);
    expect(JSON.stringify(store.getState().document)).not.toContain('hunter2');
  });

  it('closes the form without creating anything when it is cancelled', async () => {
    const user = userEvent.setup();
    const { store } = renderWithStore(<NodeConfigForm nodeId="slack" />);
    await user.click(await screen.findByRole('button', { name: 'Create new credential' }));
    await user.click(await screen.findByRole('button', { name: 'Cancel' }));
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });
    expect(store.getState().history.past).toHaveLength(0);
  });
});

describe('the credential of the WhatsApp and the AI steps', () => {
  const withNodes = (): FlowDocument => {
    const flow = exampleFlowDocument();
    flow.nodes.push(
      { id: 'inbound', name: 'WhatsApp trigger', element: 'startEvent', type: 'orcon.whatsapp.whatsappTrigger@1', parameters: {} },
      { id: 'reply', name: 'WhatsApp', element: 'serviceTask', type: 'orcon.whatsapp.whatsapp@1', parameters: { phoneNumberId: '1234567890' } },
      { id: 'agent', name: 'Sales agent', element: 'serviceTask', type: 'orcon.ai.agent@1', parameters: { model: { provider: 'acme', name: 'big-1' } } },
      { id: 'llm', name: 'Classify', element: 'serviceTask', type: 'orcon.ai.llm@1', parameters: { model: { provider: 'acme', name: 'small-1', credential: modelCredential.id }, prompt: 'Say hi' } },
    );
    return flow;
  };

  it.each([
    ['inbound', 'WhatsApp credential', 'Ventas WhatsApp', 'Agent model key'],
    ['reply', 'WhatsApp credential', 'Ventas WhatsApp', 'Agent model key'],
  ])('%s offers the WhatsApp credentials of the project and no other', async (nodeId, label, offered, notOffered) => {
    const user = userEvent.setup();
    const { store } = renderWithStore(<NodeConfigForm nodeId={nodeId} />, withNodes());
    const select = await screen.findByRole('combobox', { name: new RegExp(label) });
    await user.click(select);
    const names = within(await screen.findByRole('listbox')).getAllByRole('option').map((option) => option.textContent);
    expect(names.some((name) => name.startsWith(offered))).toBe(true);
    expect(names.some((name) => name.startsWith(notOffered))).toBe(false);
    await user.click(await screen.findByRole('option', { name: new RegExp(`^${offered}`) }));
    // The trigger and the send step both read it from `credential`; a send step keeps its other parameters.
    expect(nodeOf(store.getState().document, nodeId).parameters['credential']).toBe(whatsappCredential.id);
    expect(nodeOf(store.getState().document, nodeId)).not.toHaveProperty('credentials');
  });

  it('keeps the rest of the parameters of the send step when the credential changes', async () => {
    const user = userEvent.setup();
    const { store } = renderWithStore(<NodeConfigForm nodeId="reply" />, withNodes());
    await user.click(await screen.findByRole('combobox', { name: /WhatsApp credential/ }));
    await user.click(await screen.findByRole('option', { name: /^Ventas WhatsApp/ }));
    expect(nodeOf(store.getState().document, 'reply').parameters).toEqual({ phoneNumberId: '1234567890', credential: whatsappCredential.id });
    // The raw parameters show the choice too: the text restarts from the document.
    await waitFor(() => {
      expect(screen.getByRole<HTMLTextAreaElement>('textbox', { name: /Parameters/ }).value).toContain(whatsappCredential.id);
    });
  });

  it('puts the model credential inside the model reference and keeps its provider and name', async () => {
    const user = userEvent.setup();
    const { store } = renderWithStore(<NodeConfigForm nodeId="agent" />, withNodes());
    await user.click(await screen.findByRole('combobox', { name: /Model credential/ }));
    const names = within(await screen.findByRole('listbox')).getAllByRole('option').map((option) => option.textContent);
    expect(names.some((name) => name.startsWith('Agent model key'))).toBe(true);
    expect(names.some((name) => name.startsWith('Ventas WhatsApp'))).toBe(false);
    await user.click(await screen.findByRole('option', { name: /^Agent model key/ }));
    expect(nodeOf(store.getState().document, 'agent').parameters['model']).toEqual({ provider: 'acme', name: 'big-1', credential: modelCredential.id });
  });

  it('shows the model credential a step already has and clears only that member', async () => {
    const user = userEvent.setup();
    const { store } = renderWithStore(<NodeConfigForm nodeId="llm" />, withNodes());
    const select = await screen.findByRole('combobox', { name: /Model credential/ });
    await waitFor(() => {
      expect(select).toHaveTextContent('Agent model key');
    });
    await user.click(select);
    await user.click(await screen.findByRole('option', { name: /No credential/ }));
    expect(nodeOf(store.getState().document, 'llm').parameters).toEqual({ model: { provider: 'acme', name: 'small-1' }, prompt: 'Say hi' });
  });

  it('creates a WhatsApp credential from the step with the Meta guide and chooses it', async () => {
    const user = userEvent.setup();
    let body: unknown;
    server.use(
      http.post(`/api/v1/projects/${project.id}/credentials`, async ({ request }) => {
        body = await request.json();
        return HttpResponse.json({ ...whatsappCredential, id: '0195f1a0-0000-7000-8000-0000000000c8', name: 'Second line' }, { status: 201, headers: { ETag: '"1-1"' } });
      }),
    );
    const { store } = renderWithStore(<NodeConfigForm nodeId="inbound" />, withNodes());
    await user.click(await screen.findByRole('button', { name: 'Create new credential' }));
    const dialog = await screen.findByRole('dialog', { name: 'Create credential' });
    expect(within(dialog).queryByRole('list', { name: 'Service' })).toBeNull();
    expect(within(dialog).getByRole('complementary', { name: 'How to get these details' })).toBeVisible();
    await user.type(within(dialog).getByLabelText(/^Credential name/), 'Second line');
    await user.type(within(dialog).getByLabelText(/^Access token/), 'EAAB');
    await user.type(within(dialog).getByLabelText(/^App secret/), 'secret');
    await user.type(within(dialog).getByLabelText(/^Verify token/), 'verify');
    await user.click(within(dialog).getByRole('button', { name: 'Create credential' }));
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });
    expect(body).toMatchObject({ type: 'orcon.whatsapp@1', allowedHosts: ['graph.facebook.com'], secretFields: { token: 'EAAB', appSecret: 'secret', verifyToken: 'verify' } });
    expect(nodeOf(store.getState().document, 'inbound').parameters['credential']).toBe('0195f1a0-0000-7000-8000-0000000000c8');
    expect(JSON.stringify(store.getState().document)).not.toContain('EAAB');
  });
});

describe('the sub-flow widget', () => {
  const [notify, archive] = SAMPLE_SUBFLOWS;

  it('shows the flow that is called, links to it and warns when it has no published version', async () => {
    renderWithStore(<Call id="call" />, withCall({ target: archive.id, version: 'CURRENT', mode: 'WAIT', perItem: false }));
    expect(await screen.findByRole('combobox', { name: /Flow/ })).toHaveTextContent('Archive order');
    expect(screen.getByRole('link', { name: /Open sub-flow/ })).toHaveAttribute('href', `/orgs/acme/projects/ventas/flows/${archive.id}`);
    expect(screen.getByText(/Publish Archive order first/)).toBeInTheDocument();
  });

  it('does not warn about a flow that is published', async () => {
    renderWithStore(<Call id="call" />, withCall({ target: notify.id, version: 'CURRENT', mode: 'WAIT', perItem: false }));
    expect(await screen.findByRole('combobox', { name: /Flow/ })).toHaveTextContent('Notify the team');
    expect(screen.queryByText(/Publish .* first/)).not.toBeInTheDocument();
  });

  it('does not warn about an unpublished flow when the node calls a fixed version', async () => {
    renderWithStore(<Call id="call" />, withCall({ target: archive.id, version: 2, mode: 'WAIT', perItem: false }));
    await screen.findByRole('combobox', { name: /Flow/ });
    expect(screen.queryByText(/Publish .* first/)).not.toBeInTheDocument();
    expect(screen.getByRole('spinbutton', { name: 'Version number' })).toHaveValue(2);
  });

  it('warns when a flow would call itself', async () => {
    renderWithStore(<Call id="call" />, withCall({ target: EXAMPLE_FLOW_ID, version: 'CURRENT', mode: 'WAIT', perItem: false }));
    expect(await screen.findByText(/This flow would call itself/)).toBeInTheDocument();
  });

  it('says a flow that is not in the list is unknown', async () => {
    renderWithStore(<Call id="call" />, withCall({ target: '01930000-0000-7000-8000-0000000000ff', version: 'CURRENT', mode: 'WAIT', perItem: false }));
    expect(await screen.findByText(/This flow is not in the list/)).toBeInTheDocument();
  });

  it('chooses the flow by a formula among allowed flows, writing the block only once there is a formula', async () => {
    const user = userEvent.setup();
    const { store } = renderWithStore(<Call id="call" />, withCall({ target: notify.id, version: 'CURRENT', mode: 'WAIT', perItem: false }));
    await user.click(await screen.findByRole('button', { name: 'By a formula' }));
    // Nothing is chosen yet, so there is no call block and no option that needs one.
    expect(nodeOf(store.getState().document, 'call')).not.toHaveProperty('call');
    expect(screen.queryByRole('switch', { name: 'Call once for each item' })).not.toBeInTheDocument();
    const formula = await screen.findByRole('textbox', { name: /Flow identifier/ });
    await user.type(formula, '{{{{ $json.flow }}');
    expect(nodeOf(store.getState().document, 'call')['call']).toMatchObject({ target: '={{ $json.flow }}', version: 'CURRENT', mode: 'WAIT', perItem: false });
    await user.click(await screen.findByRole('checkbox', { name: 'Notify the team' }));
    expect(nodeOf(store.getState().document, 'call')['call']).toMatchObject({ allowedTargets: [notify.id] });
    await user.click(screen.getByRole('checkbox', { name: 'Notify the team' }));
    expect(nodeOf(store.getState().document, 'call')['call']).not.toHaveProperty('allowedTargets');
    // No link to a flow chosen by a formula.
    expect(screen.queryByRole('link', { name: /Open sub-flow/ })).not.toBeInTheDocument();
    // Emptying the formula takes the block away again.
    await user.clear(formula);
    expect(nodeOf(store.getState().document, 'call')).not.toHaveProperty('call');
  });

  it('turns per item on and off and keeps the members it does not know', async () => {
    const user = userEvent.setup();
    const { store } = renderWithStore(<Call id="call" />, withCall({ target: notify.id, version: 'CURRENT', mode: 'FIRE_AND_FORGET', perItem: false, intentionalRecursion: { maxDepth: 3 } }));
    await user.click(await screen.findByRole('switch', { name: 'Call once for each item' }));
    expect(nodeOf(store.getState().document, 'call')['call']).toEqual({ target: notify.id, version: 'CURRENT', mode: 'FIRE_AND_FORGET', perItem: true, intentionalRecursion: { maxDepth: 3 } });
  });

  it('shows a node whose call block is missing as having no flow chosen, and offers no option until one is', async () => {
    const flow = exampleFlowDocument();
    flow.nodes.push({ id: 'call', name: 'Ship', element: 'callActivity', type: 'orcon.callFlow@1', parameters: {} });
    const { store } = renderWithStore(<Call id="call" />, flow);
    expect(await screen.findByRole('combobox', { name: /Flow/ })).toHaveTextContent('Choose a flow');
    expect(screen.queryByRole('switch', { name: 'Call once for each item' })).not.toBeInTheDocument();
    expect(screen.queryByRole('combobox', { name: 'Version' })).not.toBeInTheDocument();
    expect(nodeOf(store.getState().document, 'call')).not.toHaveProperty('call');
  });

  it('is part of the form of a Call sub-flow node, with the data it sends', async () => {
    renderWithStore(<NodeConfigForm nodeId="call" />, withCall({ target: notify.id, version: 'CURRENT', mode: 'WAIT', perItem: false }));
    expect(await screen.findByRole('combobox', { name: /Flow/ })).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'Data to send' })).toBeInTheDocument();
  });
});

describe('the settings of a step', () => {
  it('writes the retry policy in the shape of the flow format and removes it when attempts are cleared', async () => {
    const user = userEvent.setup();
    const { store } = renderWithStore(<Settings id="slack" />);
    const retry = () => nodeOf(store.getState().document, 'slack')['retry'];
    expect(screen.queryByRole('textbox', { name: /Wait before the first retry/ })).not.toBeInTheDocument();
    await user.type(await screen.findByRole('textbox', { name: /Attempts in total/ }), '3');
    expect(retry()).toEqual({ maxAttempts: 3 });
    await user.type(screen.getByRole('textbox', { name: /Wait before the first retry/ }), '2');
    await user.type(screen.getByRole('textbox', { name: /Wait grows by a factor/ }), '1.5');
    await user.type(screen.getByRole('textbox', { name: /Longest wait/ }), '90');
    await user.click(screen.getByRole('switch', { name: /Retry-After/ }));
    expect(retry()).toEqual({ maxAttempts: 3, initialInterval: 'PT2S', factor: 1.5, maxInterval: 'PT1M30S', respectRetryAfter: true });
    await user.click(screen.getByRole('switch', { name: /Retry-After/ }));
    expect(retry()).not.toHaveProperty('respectRetryAfter');
    await user.clear(screen.getByRole('textbox', { name: /Attempts in total/ }));
    expect(retry()).toBeUndefined();
    expect(nodeOf(store.getState().document, 'slack')).not.toHaveProperty('retry');
  });

  it('refuses a value the format refuses and says so once the person leaves the field', async () => {
    const user = userEvent.setup();
    const { store } = renderWithStore(<Settings id="slack" />);
    const attempts = await screen.findByRole('textbox', { name: /Attempts in total/ });
    await user.type(attempts, '0');
    await user.tab();
    expect(await screen.findByText('Enter a whole number of 1 or more.')).toBeInTheDocument();
    expect(nodeOf(store.getState().document, 'slack')).not.toHaveProperty('retry');
    await user.clear(attempts);
    await user.type(attempts, '2.5');
    // The draft keeps the last valid number while the field shows the mistake.
    expect(nodeOf(store.getState().document, 'slack')['retry']).toEqual({ maxAttempts: 2 });
    expect(screen.getByText('Enter a whole number of 1 or more.')).toBeInTheDocument();
  });

  it('writes the time limits as durations and removes them when cleared', async () => {
    const user = userEvent.setup();
    const { store } = renderWithStore(<Settings id="slack" />);
    const timeouts = () => nodeOf(store.getState().document, 'slack')['timeouts'];
    await user.type(await screen.findByRole('textbox', { name: /Limit for one attempt/ }), '30');
    await user.type(screen.getByRole('textbox', { name: /Limit for the whole step/ }), '300');
    expect(timeouts()).toEqual({ attempt: 'PT30S', node: 'PT5M' });
    await user.clear(screen.getByRole('textbox', { name: /Limit for one attempt/ }));
    expect(timeouts()).toEqual({ node: 'PT5M' });
    await user.clear(screen.getByRole('textbox', { name: /Limit for the whole step/ }));
    expect(timeouts()).toBeUndefined();
  });

  it('shows durations the document holds in seconds', async () => {
    const flow = exampleFlowDocument();
    Object.assign(nodeOf(flow, 'slack'), { timeouts: { attempt: 'PT1H30M' }, retry: { maxAttempts: 4, initialInterval: 'PT2S', factor: 2, maxInterval: 'PT5M' } });
    renderWithStore(<Settings id="slack" />, flow);
    expect(await screen.findByRole('textbox', { name: /Limit for one attempt/ })).toHaveValue('5400');
    expect(screen.getByRole('textbox', { name: /Attempts in total/ })).toHaveValue('4');
    expect(screen.getByRole('textbox', { name: /Longest wait/ })).toHaveValue('300');
  });

  it('writes the concurrency as a number or as ALL, in either language of the word', async () => {
    const user = userEvent.setup();
    const { store } = renderWithStore(<Settings id="slack" />);
    const concurrency = () => nodeOf(store.getState().document, 'slack')['concurrency'];
    const field = await screen.findByRole('textbox', { name: /Items at the same time/ });
    await user.type(field, '4');
    expect(concurrency()).toBe(4);
    await user.clear(field);
    await user.type(field, 'All');
    expect(concurrency()).toBe('ALL');
    await user.clear(field);
    await user.type(field, 'todos');
    expect(concurrency()).toBe('ALL');
    await user.clear(field);
    expect(concurrency()).toBeUndefined();
  });

  it('follows the draft when an edit is undone', async () => {
    const user = userEvent.setup();
    const { store } = renderWithStore(<Settings id="slack" />);
    const attempts = await screen.findByRole('textbox', { name: /Attempts in total/ });
    await user.type(attempts, '3');
    expect(attempts).toHaveValue('3');
    act(() => {
      store.getState().undo();
    });
    expect(attempts).toHaveValue('');
    act(() => {
      store.getState().redo();
    });
    expect(attempts).toHaveValue('3');
  });

  it('explains the retry default of the kind of step', async () => {
    renderWithStore(<Settings id="inventory" />);
    expect(await screen.findByText(/reading is safe to repeat/)).toBeInTheDocument();
  });

  it('explains that a step that writes is not retried by default and cannot be silenced', async () => {
    renderWithStore(<Settings id="slack" />);
    expect(await screen.findByText(/Default: no retry/)).toBeInTheDocument();
    expect(screen.getByText(/cannot be silenced/)).toBeInTheDocument();
    expect(screen.getByRole('combobox', { name: /Tell people/ })).toBeInTheDocument();
  });

  it('offers the notification setting to a step that waits and not to one that neither waits nor writes', async () => {
    const flow = exampleFlowDocument();
    flow.nodes.push({ id: 'wait', name: 'Wait for approval', element: 'userTask', type: 'acme.approval@1', parameters: {} });
    renderWithStore(<Settings id="wait" />, flow);
    expect(await screen.findByRole('combobox', { name: /Tell people/ })).toBeInTheDocument();
    expect(screen.queryByText(/cannot be silenced/)).not.toBeInTheDocument();
  });

  it('does not offer it to a step that only checks a condition', async () => {
    renderWithStore(<Settings id="validate" />);
    await screen.findByRole('textbox', { name: /Attempts in total/ });
    expect(screen.queryByRole('combobox', { name: /Tell people/ })).not.toBeInTheDocument();
  });

  it('shows the error output hint when failures go to their own output', async () => {
    const flow = exampleFlowDocument();
    Object.assign(nodeOf(flow, 'slack'), { onError: 'ERROR_OUTPUT' });
    renderWithStore(<Settings id="slack" />, flow);
    expect(await screen.findByText(/second output, “error”/)).toBeInTheDocument();
    expect(screen.getByRole('combobox', { name: /After the last attempt fails/ })).toHaveTextContent('Send failed items to an error output');
  });
});

describe('the note of a step', () => {
  it('is kept beside the position, as one undo step, and removed when emptied', async () => {
    const user = userEvent.setup();
    const { store } = renderWithStore(<Settings id="slack" />);
    const field = await screen.findByRole('textbox', { name: 'Note on this step' });
    expect(field).toHaveValue('');
    await user.type(field, 'Ask Ana');
    const entry = () => store.getState().document.editor.nodes?.['slack'];
    expect(entry()).toEqual({ x: 640, y: 60, note: 'Ask Ana' });
    expect(store.getState().history.past).toHaveLength(1);
    await user.clear(field);
    expect(entry()).toEqual({ x: 640, y: 60 });
    act(() => {
      store.getState().undo();
    });
    // Typing and clearing in one focus are one step: undo goes back to no note.
    expect(entry()).toEqual({ x: 640, y: 60 });
    expect(field).toHaveValue('');
  });

  it('says the limit and stops there', async () => {
    renderWithStore(<Settings id="slack" />);
    const field = await screen.findByRole('textbox', { name: 'Note on this step' });
    expect(field).toHaveAttribute('maxlength', '5000');
    expect(screen.getByText(/up to 5000 characters/)).toBeInTheDocument();
  });
});

describe('the connector update', () => {
  it('offers the newest version with a changelog that opens on demand', async () => {
    const user = userEvent.setup();
    renderWithStore(<Update id="slack" />);
    expect(await screen.findByText('New version available')).toBeInTheDocument();
    expect(screen.getByText('This step uses version 1.0.0. Version 1.2.0 is installed.')).toBeInTheDocument();
    expect(screen.queryByText('Version 1.2.0')).not.toBeInTheDocument();
    const toggle = screen.getByRole('button', { name: 'What changed' });
    expect(toggle).toHaveAttribute('aria-expanded', 'false');
    await user.click(toggle);
    expect(toggle).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getByRole('heading', { name: 'Version 1.1.0' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Version 1.2.0' })).toBeInTheDocument();
    // A change that can alter behaviour says so in words, not by color.
    expect(screen.getAllByText('Check after updating')).toHaveLength(1);
    expect(screen.getByText(/Redirects to another host are no longer followed/)).toBeInTheDocument();
  });

  it('updates the draft as one history entry, tells the person and goes away', async () => {
    const user = userEvent.setup();
    const { store, channel } = renderWithStore(<Update id="slack" />);
    await user.click(await screen.findByRole('button', { name: 'Update this node' }));
    expect(nodeOf(store.getState().document, 'slack')['connector']).toMatchObject({ version: '1.2.0' });
    expect(store.getState().history.past).toHaveLength(1);
    expect(channel.current()?.message).toBe('Slack now uses version 1.2.0.');
    await waitFor(() => {
      expect(screen.queryByText('New version available')).not.toBeInTheDocument();
    });
    act(() => {
      store.getState().undo();
    });
    expect(await screen.findByText('New version available')).toBeInTheDocument();
  });

  it('shows nothing for a node that is up to date', () => {
    const { container } = renderWithStore(<Update id="inventory" />);
    expect(container).not.toHaveTextContent('New version available');
  });
});

describe('the Docs tab', () => {
  it('shows what the type does, its operations, its credential notes and its parameters', async () => {
    renderWithStore(<Docs id="slack" />);
    expect(await screen.findByText(/Calls any web address and passes the response/)).toBeInTheDocument();
    expect(screen.getByText('Type orcon.http@1, version 1.0.0')).toBeInTheDocument();
    expect(screen.getByText(/Send one request per item/)).toBeInTheDocument();
    expect(screen.getByText(/A header credential adds a header/)).toBeInTheDocument();
    const parameters = screen.getByRole('heading', { name: 'Parameters' }).closest('section');
    expect(parameters).not.toBeNull();
    expect(within(parameters as HTMLElement).getByText('URL')).toBeInTheDocument();
    // The node is pinned to 1.0.0: a parameter of a later version is not documented for it.
    expect(within(parameters as HTMLElement).queryByText('Response format')).not.toBeInTheDocument();
  });

  it('says there is no documentation for a type it does not know', async () => {
    const flow = exampleFlowDocument();
    flow.nodes.push({ id: 'acme', name: 'Acme', element: 'task', type: 'acme.thing@2', parameters: {} });
    renderWithStore(<Docs id="acme" />, flow);
    expect(await screen.findByText('There is no documentation for this type of step yet.')).toBeInTheDocument();
    expect(screen.getByText('Type acme.thing@2')).toBeInTheDocument();
  });
});
