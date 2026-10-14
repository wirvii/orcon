// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { act, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { EXAMPLE_FLOW_ID, exampleFlowDocument, SAMPLE_SUBFLOWS } from '../../model/example-flow';
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
  it('lists the credentials of a matching type with the one the node uses chosen', async () => {
    renderWithStore(<NodeConfigForm nodeId="slack" />);
    const select = await screen.findByRole('combobox', { name: /Authentication/ });
    await waitFor(() => {
      expect(select).toHaveTextContent('Slack incoming webhook');
    });
    // Secret values are never shown, and the note says so.
    expect(screen.getByText(/Its value is never shown/)).toBeInTheDocument();
  });

  it('says a credential the project does not have is missing and offers two ways out', async () => {
    const user = userEvent.setup();
    const { store } = renderWithStore(<NodeConfigForm nodeId="slack" />);
    await screen.findByRole('combobox', { name: /Authentication/ });
    act(() => {
      store.getState().setCredential('slack', 'auth', 'Gone');
    });
    expect(await screen.findByText('Missing credential: Gone')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Create' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Choose another' }));
    expect(nodeOf(store.getState().document, 'slack')).not.toHaveProperty('credentials');
    await waitFor(() => {
      expect(screen.queryByText('Missing credential: Gone')).not.toBeInTheDocument();
    });
  });

  it('creates a credential from the node, chooses it and keeps no secret anywhere', async () => {
    const user = userEvent.setup();
    const { store, channel, queryClient } = renderWithStore(<NodeConfigForm nodeId="slack" />);
    await user.click(await screen.findByRole('button', { name: 'Create new credential' }));
    const dialog = await screen.findByRole('dialog', { name: 'Create credential' });
    // The type is fixed by the slot, and the fields come from the type.
    expect(within(dialog).getByRole('textbox', { name: /Header name/ })).toHaveValue('Authorization');
    const secret = within(dialog).getByLabelText(/Header value/);
    expect(secret).toHaveAttribute('type', 'password');
    expect(secret).toHaveValue('');
    await user.type(within(dialog).getByRole('textbox', { name: /^Name/ }), 'Billing key');
    await user.type(secret, 'hunter2');
    await user.click(within(dialog).getByRole('button', { name: 'Create credential' }));
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });
    expect(nodeOf(store.getState().document, 'slack')['credentials']).toEqual({ auth: { name: 'Billing key' } });
    expect(channel.current()?.message).toBe('Credential Billing key created and chosen.');
    // The mutation holds no variables after it ends, and no draft carries the secret.
    expect(queryClient.getMutationCache().getAll()).toHaveLength(0);
    expect(JSON.stringify(store.getState().document)).not.toContain('hunter2');
  });

  it('asks for the fields that are required before it creates anything', async () => {
    const user = userEvent.setup();
    const { store } = renderWithStore(<NodeConfigForm nodeId="slack" />);
    await user.click(await screen.findByRole('button', { name: 'Create new credential' }));
    const dialog = await screen.findByRole('dialog', { name: 'Create credential' });
    await user.click(within(dialog).getByRole('button', { name: 'Create credential' }));
    expect((await within(dialog).findAllByText('This field is required.')).length).toBeGreaterThanOrEqual(2);
    expect(nodeOf(store.getState().document, 'slack')['credentials']).toEqual({ auth: { name: 'Slack incoming webhook' } });
  });

  it('says the name is taken when the project already has it', async () => {
    const user = userEvent.setup();
    renderWithStore(<NodeConfigForm nodeId="slack" />);
    await user.click(await screen.findByRole('button', { name: 'Create new credential' }));
    const dialog = await screen.findByRole('dialog', { name: 'Create credential' });
    await user.type(within(dialog).getByRole('textbox', { name: /^Name/ }), 'Inventory API key');
    await user.type(within(dialog).getByLabelText(/Header value/), 'x');
    await user.click(within(dialog).getByRole('button', { name: 'Create credential' }));
    expect(await within(dialog).findByText('The project already has a credential with this name.')).toBeInTheDocument();
    expect(screen.getByRole('dialog')).toBeInTheDocument();
  });

  it('starts the form with the missing name when "Create" answers a missing credential', async () => {
    const user = userEvent.setup();
    const { store } = renderWithStore(<NodeConfigForm nodeId="slack" />);
    await screen.findByRole('combobox', { name: /Authentication/ });
    act(() => {
      store.getState().setCredential('slack', 'auth', 'Gone');
    });
    await user.click(await screen.findByRole('button', { name: 'Create' }));
    expect(await screen.findByRole('textbox', { name: /^Name/ })).toHaveValue('Gone');
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

describe('the sub-flow widget', () => {
  const [notify, archive] = SAMPLE_SUBFLOWS;

  it('shows the flow that is called, links to it and warns when it has no published version', async () => {
    renderWithStore(<Call id="call" />, withCall({ target: archive.id, version: 'CURRENT', mode: 'WAIT', perItem: false }));
    expect(await screen.findByRole('combobox', { name: /Flow/ })).toHaveTextContent('Archive order');
    expect(screen.getByRole('link', { name: /Open sub-flow/ })).toHaveAttribute('href', `/flows/${archive.id}`);
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

  it('chooses the flow by a formula among allowed flows', async () => {
    const user = userEvent.setup();
    const { store } = renderWithStore(<Call id="call" />, withCall({ target: notify.id, version: 'CURRENT', mode: 'WAIT', perItem: false }));
    await user.click(await screen.findByRole('button', { name: 'By a formula' }));
    expect(nodeOf(store.getState().document, 'call')['call']).toMatchObject({ target: '=' });
    const formula = await screen.findByRole('textbox', { name: /Flow identifier/ });
    await user.type(formula, '{{{{ $json.flow }}');
    expect(nodeOf(store.getState().document, 'call')['call']).toMatchObject({ target: '={{ $json.flow }}' });
    await user.click(await screen.findByRole('checkbox', { name: 'Notify the team' }));
    expect(nodeOf(store.getState().document, 'call')['call']).toMatchObject({ allowedTargets: [notify.id] });
    await user.click(screen.getByRole('checkbox', { name: 'Notify the team' }));
    expect(nodeOf(store.getState().document, 'call')['call']).not.toHaveProperty('allowedTargets');
    // No link to a flow chosen by a formula.
    expect(screen.queryByRole('link', { name: /Open sub-flow/ })).not.toBeInTheDocument();
  });

  it('turns per item on and off and keeps the members it does not know', async () => {
    const user = userEvent.setup();
    const { store } = renderWithStore(<Call id="call" />, withCall({ target: notify.id, version: 'CURRENT', mode: 'FIRE_AND_FORGET', perItem: false, intentionalRecursion: { maxDepth: 3 } }));
    await user.click(await screen.findByRole('switch', { name: 'Call once for each item' }));
    expect(nodeOf(store.getState().document, 'call')['call']).toEqual({ target: notify.id, version: 'CURRENT', mode: 'FIRE_AND_FORGET', perItem: true, intentionalRecursion: { maxDepth: 3 } });
  });

  it('reads a node whose call block is missing as the defaults of the format', async () => {
    const flow = exampleFlowDocument();
    flow.nodes.push({ id: 'call', name: 'Ship', element: 'callActivity', type: 'orcon.callFlow@1', parameters: {} });
    renderWithStore(<Call id="call" />, flow);
    expect(await screen.findByRole('combobox', { name: /Flow/ })).toHaveTextContent('Choose a flow');
    expect(screen.getByRole('switch', { name: 'Call once for each item' })).not.toBeChecked();
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
