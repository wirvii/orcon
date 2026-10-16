// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { createRef, useState } from 'react';
import { expect, it } from 'vitest';

import { server } from '../../../../../test/screen-harness';
import { NodeDetails } from '../../components/NodeDetails';
import type { DetailsTab } from '@orcon/ui';
import { EXAMPLE_FLOW_ID } from '../../model/example-flow';

import { DuplicateProtectionBox } from './DuplicateProtectionBox';
import type { FieldInsertController } from './field-insert';
import { renderWithStore } from './harness';

const PROJECT = '0195f1a0-0000-7000-8000-0000000000d1';
const RUN = '0195f1a0-0000-7000-8000-0000000000e1';

/** The last run of the sample flow: the HTTP request answered a customer, and the step after it received that. */
function serveLastRun() {
  server.use(
    http.get(`/api/v1/flows/${EXAMPLE_FLOW_ID}`, () =>
      HttpResponse.json({ id: EXAMPLE_FLOW_ID, projectId: PROJECT, activationState: 'INACTIVE', activeVersion: null, hasUnpublishedChanges: true, draftRevision: 1 }, { headers: { ETag: '"1"' } }),
    ),
    http.get('/api/v1/projects/:projectId/executions', () =>
      HttpResponse.json({ items: [{ id: RUN, flowId: EXAMPLE_FLOW_ID, projectId: PROJECT, flowVersion: 1, mode: 'MANUAL', status: 'SUCCEEDED', createdAt: '2026-10-15T10:41:00Z', dataHidden: false }] }),
    ),
    http.get(`/api/v1/executions/${RUN}/nodes`, () => HttpResponse.json({ items: [] })),
    http.get(`/api/v1/executions/${RUN}/data`, () =>
      HttpResponse.json({
        items: [
          {
            nodeExecutionId: '0195f1a0-0000-7000-8000-000000000101',
            nodeId: 'slack',
            side: 'INPUT',
            port: 'main',
            itemCount: 1,
            dataItems: [{ customer: { name: 'Camila Rojas', phone: '573104567890' }, text: 'Hello' }],
          },
          {
            nodeExecutionId: '0195f1a0-0000-7000-8000-000000000101',
            nodeId: 'slack',
            side: 'OUTPUT',
            port: 'main',
            itemCount: 1,
            dataItems: [{ status: 200, ok: true }],
          },
        ],
      }),
    ),
  );
}

/** The details view with its tab kept by the page, the way the editor keeps it. */
function Details({ nodeId, controller }: { nodeId: string; controller: { current: FieldInsertController | undefined } }) {
  const [tab, setTab] = useState<DetailsTab>('parameters');
  return <NodeDetails nodeId={nodeId} layout="dialog" tab={tab} onTabChange={setTab} onClose={() => undefined} onReturnFocus={() => undefined} flowId={EXAMPLE_FLOW_ID} insertController={controller} />;
}

function renderDetails(view: 'chips' | 'raw' = 'chips', nodeId = 'slack') {
  serveLastRun();
  const controller = createRef<FieldInsertController | undefined>() as { current: FieldInsertController | undefined };
  const harness = renderWithStore(
    <Details nodeId={nodeId} controller={controller} />,
    undefined,
    view,
  );
  return { ...harness, controller };
}

/** The data a drag carries, the way a browser keeps it between the start and the drop. */
function dragData() {
  const held = new Map<string, string>();
  return {
    types: [] as string[],
    effectAllowed: 'all',
    dropEffect: 'none',
    setData(type: string, value: string) {
      held.set(type, value);
      this.types = [...held.keys()];
    },
    getData: (type: string) => held.get(type) ?? '',
  };
}

it('node details shows input parameters and output panels', async () => {
  renderDetails();
  const dialog = await screen.findByRole('dialog', { name: 'Slack details' });
  for (const pane of ['input', 'parameters', 'output']) expect(dialog.querySelector(`[data-pane="${pane}"]`)).not.toBeNull();
  // The input starts on its fields, in the order Schema, Table, JSON; the output on its table.
  const input = dialog.querySelector<HTMLElement>('[data-pane="input"]');
  const output = dialog.querySelector<HTMLElement>('[data-pane="output"]');
  expect(within(input ?? dialog).getAllByRole('button', { pressed: true })[0]).toHaveTextContent('Schema');
  expect(within(input ?? dialog).getAllByRole('button', { name: /^(Schema|Table|JSON)$/ }).map((button) => button.textContent)).toEqual(['Schema', 'Table', 'JSON']);
  expect(await within(input ?? dialog).findByRole('button', { name: /Use customer\.phone in a parameter/ })).toBeVisible();
  expect(await within(output ?? dialog).findByText('200')).toBeVisible();
  // The step knows its name, its documentation and how to run it.
  expect(within(dialog).getByRole('button', { name: 'Documentation' })).toBeVisible();
  expect(within(dialog).getByRole('button', { name: 'Execute step' })).toBeVisible();
});

it('node details panels resize with the keyboard', async () => {
  renderDetails();
  const dialog = await screen.findByRole('dialog', { name: 'Slack details' });
  const handle = within(dialog).getByRole('separator', { name: 'Resize the input and parameters panes' });
  const before = Number(handle.getAttribute('aria-valuenow'));
  handle.focus();
  await userEvent.keyboard('{ArrowLeft}');
  expect(Number(handle.getAttribute('aria-valuenow'))).not.toBe(before);
  expect(within(dialog).getByRole('separator', { name: 'Resize the parameters and output panes' })).toBeVisible();
});

it('dragging an input field inserts the expression as a chip', async () => {
  const { store } = renderDetails();
  const dialog = await screen.findByRole('dialog', { name: 'Slack details' });
  const field = await within(dialog).findByRole('button', { name: /Use customer\.phone in a parameter/ });
  const transfer = dragData();
  fireEvent.dragStart(field, { dataTransfer: transfer });
  expect(transfer.types).toEqual(['application/x-orcon-field']);
  const target = within(dialog).getByRole('textbox', { name: /URL/ });
  fireEvent.dragOver(target, { dataTransfer: transfer });
  fireEvent.drop(target, { dataTransfer: transfer });
  // A fixed text that receives a field becomes a formula that keeps its text; what is stored is the usual equals-prefixed string.
  const stored = store.getState().document.nodes.find((node) => node.id === 'slack')?.parameters['url'];
  expect(stored).toBe('=https://hooks.slack.example.test/services/orders{{ $json.customer.phone }}');
  // The field now reads as a chip, with its text around it.
  const chips = await within(dialog).findByRole('group', { name: 'URL: formula as chips' });
  expect(within(chips).getByText('customer.phone')).toBeVisible();
  expect(chips).toHaveTextContent('https://hooks.slack.example.test/services/orders');
});

it('chooses an input field with Enter and puts it in the parameter that had the focus', async () => {
  const { store } = renderDetails('raw');
  const dialog = await screen.findByRole('dialog', { name: 'Slack details' });
  await userEvent.click(within(within(dialog).getByRole('group', { name: 'Kind of value of URL' })).getByRole('button', { name: 'Expression' }));
  await userEvent.click(within(dialog).getByRole('textbox', { name: /URL/ }));
  const field = await within(dialog).findByRole('button', { name: /Use customer\.name in a parameter/ });
  field.focus();
  await userEvent.keyboard('{Enter}');
  await waitFor(() => {
    expect(String(store.getState().document.nodes.find((node) => node.id === 'slack')?.parameters['url'])).toMatch(/\{\{ \$json\.customer\.name \}\}$/);
  });
});

it('raw toggle switches a field to raw text with one click and is remembered', async () => {
  const first = renderDetails('chips', 'validate');
  let dialog = await screen.findByRole('dialog', { name: 'Validate order details' });
  // The condition is an expression that is not a plain path: it is raw text inside a chip.
  const chips = await within(dialog).findByRole('group', { name: 'Condition: formula as chips' });
  expect(within(chips).getByText('$json.total > 0')).toBeVisible();
  expect(within(dialog).queryByRole('textbox', { name: /Condition/ })).toBeNull();
  await userEvent.click(within(dialog).getByRole('button', { name: 'View as text' }));
  expect(within(dialog).getByRole('textbox', { name: /Condition/ })).toHaveValue('{{ $json.total > 0 }}');
  // The way back is one click too.
  expect(within(dialog).getByRole('button', { name: 'View as chips' })).toHaveAttribute('aria-pressed', 'true');
  first.unmount();

  // Opened again, the editor remembers the choice: the field shows as text without asking.
  window.localStorage.setItem('orcon.editor.expression-view', JSON.stringify('raw'));
  renderDetails('raw', 'validate');
  dialog = await screen.findByRole('dialog', { name: 'Validate order details' });
  expect(await within(dialog).findByRole('textbox', { name: /Condition/ })).toHaveValue('{{ $json.total > 0 }}');
});

it('ask AI button is hidden', async () => {
  renderDetails();
  const dialog = await screen.findByRole('dialog', { name: 'Slack details' });
  await within(dialog).findByRole('textbox', { name: /URL/ });
  // The AI builder has no server part yet: nothing in the details offers it, and no placeholder pretends.
  expect(within(dialog).queryByText(/ask (the )?ai|pedir a la ia/i)).toBeNull();
  expect(within(dialog).queryByRole('button', { name: /ask (the )?ai|pedir a la ia/i })).toBeNull();
});

it('credential shows name and type only without health data', async () => {
  renderDetails();
  const dialog = await screen.findByRole('dialog', { name: 'Slack details' });
  const card = await within(dialog).findByRole('group', { name: /^Credential in use: Slack incoming webhook, / });
  expect(card).toHaveTextContent('Slack incoming webhook');
  // The credential service does not give an account or a health yet: the card says no more than it knows.
  expect(card.textContent).not.toMatch(/connected|expires|failed|untested|tested|used in|flows/i);
});

it('duplicate protection box shows the unique key for idempotency key steps', async () => {
  renderWithStore(<DuplicateProtectionBox safety={{ purity: 'EFFECT', safetyLevel: 'IDEMPOTENCY_KEY', key: 'message.wamid' }} service="WhatsApp" />);
  expect(await screen.findByText('Protection against duplicate messages')).toBeVisible();
  expect(screen.getByText(/WhatsApp ignores a repeat/)).toBeVisible();
  expect(screen.getByText('message.wamid')).toBeVisible();
});

it('duplicate protection box warns for unsafe steps', async () => {
  renderDetails();
  const dialog = await screen.findByRole('dialog', { name: 'Slack details' });
  // The Slack step is an HTTP POST: it changes something outside and cannot confirm that it already happened.
  expect(await within(dialog).findByText('A retry can send this twice')).toBeVisible();
  expect(within(dialog).queryByText('Protection against duplicate messages')).toBeNull();
  // A request that only reads has no box.
  const reads = renderDetails('chips', 'inventory');
  const second = await screen.findAllByRole('dialog', { name: 'HTTP Request details' });
  expect(within(second[0] ?? dialog).queryByText('A retry can send this twice')).toBeNull();
  reads.unmount();
});

it('retries and safety tab shows the retry settings', async () => {
  renderDetails();
  const dialog = await screen.findByRole('dialog', { name: 'Slack details' });
  expect(within(dialog).queryByRole('textbox', { name: /Attempts in total/ })).toBeNull();
  await userEvent.click(within(dialog).getByRole('tab', { name: 'Retries and safety' }));
  expect(await within(dialog).findByRole('textbox', { name: /Attempts in total/ })).toBeVisible();
  expect(within(dialog).getByText('A retry can send this twice')).toBeVisible();
  // The time limits and the rest stay in Settings.
  await userEvent.click(within(dialog).getByRole('tab', { name: 'Settings' }));
  expect(await within(dialog).findByRole('textbox', { name: /Limit for one attempt/ })).toBeVisible();
  expect(within(dialog).queryByRole('textbox', { name: /Attempts in total/ })).toBeNull();
});
