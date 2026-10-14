// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { act, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { exampleFlowDocument } from '../../model/example-flow';
import { findNode, type FlowDocument } from '../../model/flow-document';
import type { NodeManifest } from '../../model/node-manifests';

import { renderWithStore } from './harness';
import { NodeConfigForm } from './NodeConfigForm';

afterEach(() => {
  vi.restoreAllMocks();
});

const parametersOf = (document: FlowDocument, id: string) => findNode(document, id)?.parameters ?? {};

/** A schema with one parameter of every type, to prove the renderer has a widget for each. */
const everyType: NodeManifest = {
  key: 'every',
  type: 'acme.every@1',
  parameters: [
    { name: 'title', type: 'STRING', required: true },
    { name: 'limit', type: 'NUMBER' },
    { name: 'enabled', type: 'BOOLEAN' },
    { name: 'mode', type: 'OPTIONS', default: 'a', options: ['a', 'b'] },
    { name: 'tags', type: 'MULTI_OPTIONS', options: ['red', 'green', 'blue'] },
    { name: 'pairs', type: 'FIXED_COLLECTION', fields: [{ name: 'name', type: 'STRING', required: true }, { name: 'value', type: 'STRING' }] },
    { name: 'payload', type: 'JSON' },
    { name: 'when', type: 'DATE_TIME' },
    { name: 'target', type: 'RESOURCE_LOCATOR' },
    { name: 'file', type: 'BINARY_PROPERTY', default: 'data' },
    { name: 'extra', type: 'STRING', displayWhen: [{ param: 'enabled', oneOf: [true] }] },
  ],
  credentials: [],
  effect: 'pure',
  operations: [],
  catalog: {
    en: {
      'param.title.label': 'Title',
      'param.limit.label': 'Limit',
      'param.enabled.label': 'Enabled',
      'param.mode.label': 'Mode',
      'param.mode.option.a': 'Mode A',
      'param.mode.option.b': 'Mode B',
      'param.tags.label': 'Tags',
      'param.tags.option.red': 'Red',
      'param.tags.option.green': 'Green',
      'param.tags.option.blue': 'Blue',
      'param.pairs.label': 'Pairs',
      'param.pairs.field.name': 'Pair name',
      'param.pairs.field.value': 'Pair value',
      'param.payload.label': 'Payload',
      'param.when.label': 'When',
      'param.target.label': 'Target',
      'param.file.label': 'File property',
      'param.extra.label': 'Extra',
    },
    es: {},
  },
};

const flowWithEveryNode = () => {
  const flow = exampleFlowDocument();
  flow.nodes.push({ id: 'every', name: 'Every', element: 'task', type: 'acme.every@1', parameters: {} });
  return flow;
};

describe('the node configuration form', () => {
  describe('a node of the sample flow', () => {
    it('draws the fields of the HTTP Request from its schema, with the pinned version', async () => {
      renderWithStore(<NodeConfigForm nodeId="slack" />);
      expect(await screen.findByRole('textbox', { name: /URL/ })).toHaveValue('https://hooks.slack.example.test/services/orders');
      expect(screen.getByRole('combobox', { name: /Method/ })).toHaveTextContent('POST');
      // POST sends a body; the body is an expression, shown as text.
      expect(screen.getByRole('textbox', { name: /Body/ })).toHaveValue(`{{ { text: "Order " + $('Webhook').item.json.id + ": " + $('HTTP Request').item.json.stock + " in stock" } }}`);
      // The node is pinned to 1.0.0: the options of later versions are not there.
      expect(screen.queryByText('Response format')).not.toBeInTheDocument();
      expect(screen.queryByText('Follow redirects to other hosts')).not.toBeInTheDocument();
    });

    it('shows the options a newer pinned version has', async () => {
      renderWithStore(<NodeConfigForm nodeId="inventory" />);
      expect(await screen.findByText('Response format')).toBeInTheDocument();
      expect(screen.getByRole('switch', { name: 'Follow redirects to other hosts' })).not.toBeChecked();
    });

    it('hides the body when the method does not send one', async () => {
      const { store } = renderWithStore(<NodeConfigForm nodeId="inventory" />);
      expect(await screen.findByRole('textbox', { name: /URL/ })).toBeInTheDocument();
      expect(screen.queryByRole('textbox', { name: /Body/ })).not.toBeInTheDocument();
      act(() => {
        store.getState().setParameter('inventory', 'method', 'POST');
      });
      expect(await screen.findByRole('textbox', { name: /Body/ })).toBeInTheDocument();
    });

    it('writes what is typed to the draft, as one undo step', async () => {
      const user = userEvent.setup();
      const { store } = renderWithStore(<NodeConfigForm nodeId="slack" />);
      const url = await screen.findByRole('textbox', { name: /URL/ });
      await user.clear(url);
      await user.type(url, 'https://example.test/a');
      expect(parametersOf(store.getState().document, 'slack')['url']).toBe('https://example.test/a');
      expect(store.getState().history.past).toHaveLength(1);
      act(() => {
        store.getState().undo();
      });
      expect(parametersOf(store.getState().document, 'slack')['url']).toBe('https://hooks.slack.example.test/services/orders');
    });

    it('starts a new undo step each time the field is focused again', async () => {
      const user = userEvent.setup();
      const { store } = renderWithStore(<NodeConfigForm nodeId="slack" />);
      const url = await screen.findByRole('textbox', { name: /URL/ });
      await user.type(url, 'x');
      await user.tab();
      await user.click(url);
      await user.type(url, 'y');
      expect(store.getState().history.past).toHaveLength(2);
    });

    it('switches a field between fixed and expression and keeps the text', async () => {
      const user = userEvent.setup();
      const { store } = renderWithStore(<NodeConfigForm nodeId="slack" />);
      await screen.findByRole('textbox', { name: /URL/ });
      const kind = screen.getByRole('group', { name: 'Kind of value of URL' });
      expect(within(kind).getByRole('button', { name: 'Fixed' })).toHaveAttribute('aria-pressed', 'true');
      await user.click(within(kind).getByRole('button', { name: 'Expression' }));
      expect(parametersOf(store.getState().document, 'slack')['url']).toBe('=https://hooks.slack.example.test/services/orders');
      expect(within(kind).getByRole('button', { name: 'Expression' })).toHaveAttribute('aria-pressed', 'true');
      // An expression is typed as text and stored with its equals sign.
      const url = screen.getByRole('textbox', { name: /URL/ });
      await user.clear(url);
      await user.type(url, '{{{{ $json.hook }}');
      expect(parametersOf(store.getState().document, 'slack')['url']).toBe('={{ $json.hook }}');
      await user.click(within(kind).getByRole('button', { name: 'Fixed' }));
      expect(parametersOf(store.getState().document, 'slack')['url']).toBe('{{ $json.hook }}');
    });

    it('starts the condition of an If in expression mode and shows it as text', async () => {
      renderWithStore(<NodeConfigForm nodeId="validate" />);
      expect(await screen.findByRole('textbox', { name: /Condition/ })).toHaveValue('{{ $json.total > 0 }}');
      expect(within(screen.getByRole('group', { name: 'Kind of value of Condition' })).getByRole('button', { name: 'Expression' })).toHaveAttribute('aria-pressed', 'true');
    });

    it('says a required field is empty once the person leaves it', async () => {
      const user = userEvent.setup();
      renderWithStore(<NodeConfigForm nodeId="slack" />);
      const url = await screen.findByRole('textbox', { name: /URL/ });
      await user.clear(url);
      expect(screen.queryByText('Fill in this field.')).not.toBeInTheDocument();
      await user.tab();
      expect(await screen.findByText('Fill in this field.')).toBeInTheDocument();
      expect(url).toBeInvalid();
    });

    it('has nothing to set for a step without parameters', async () => {
      renderWithStore(<NodeConfigForm nodeId="done" />);
      expect(await screen.findByText('This step has nothing to set here.')).toBeInTheDocument();
    });

    it('explains a step that came from an import and has no equivalent', async () => {
      const flow = exampleFlowDocument();
      flow.nodes.push({ id: 'odd', name: 'Odd', element: 'task', type: 'orcon.unsupported@1', parameters: {} });
      renderWithStore(<NodeConfigForm nodeId="odd" />, flow);
      expect(await screen.findByText('This step is not supported')).toBeInTheDocument();
    });
  });

  describe('a step type the editor has no schema for', () => {
    it('shows the parameters as JSON and writes valid edits', async () => {
      const user = userEvent.setup();
      const flow = exampleFlowDocument();
      flow.nodes.push({ id: 'acme', name: 'Acme', element: 'task', type: 'acme.thing@1', parameters: { a: 1 } });
      const { store } = renderWithStore(<NodeConfigForm nodeId="acme" />, flow);
      const box = await screen.findByRole('textbox', { name: 'Parameters (JSON)' });
      expect(screen.getByText('No form for this type of step')).toBeInTheDocument();
      expect(box).toHaveValue('{\n  "a": 1\n}');
      await user.clear(box);
      await user.click(box);
      await user.paste('{"b": true}');
      expect(parametersOf(store.getState().document, 'acme')).toEqual({ b: true });
    });

    it('says the text is not valid JSON and writes nothing', async () => {
      const user = userEvent.setup();
      const flow = exampleFlowDocument();
      flow.nodes.push({ id: 'acme', name: 'Acme', element: 'task', type: 'acme.thing@1', parameters: { a: 1 } });
      const { store } = renderWithStore(<NodeConfigForm nodeId="acme" />, flow);
      const box = await screen.findByRole('textbox', { name: 'Parameters (JSON)' });
      await user.type(box, '{{');
      expect(await screen.findByText('This is not valid JSON.')).toBeInTheDocument();
      expect(parametersOf(store.getState().document, 'acme')).toEqual({ a: 1 });
    });
  });

  describe('lists of pairs', () => {
    it('adds, edits and removes entries', async () => {
      const user = userEvent.setup();
      const { store } = renderWithStore(<NodeConfigForm nodeId="slack" />);
      await user.click(await screen.findByRole('button', { name: 'Add to Headers' }));
      expect(parametersOf(store.getState().document, 'slack')['headers']).toEqual([{ name: '', value: '' }]);
      await user.type(screen.getByRole('textbox', { name: 'Header name, 1' }), 'X-Trace');
      await user.type(screen.getByRole('textbox', { name: 'Header value, 1' }), '={{{{ $json.id }}');
      expect(parametersOf(store.getState().document, 'slack')['headers']).toEqual([{ name: 'X-Trace', value: '={{ $json.id }}' }]);
      await user.click(screen.getByRole('button', { name: 'Remove Headers, entry 1' }));
      expect(parametersOf(store.getState().document, 'slack')['headers']).toEqual([]);
    });

    it('says an entry needs a name once the person leaves it', async () => {
      const user = userEvent.setup();
      renderWithStore(<NodeConfigForm nodeId="slack" />);
      await user.click(await screen.findByRole('button', { name: 'Add to Headers' }));
      await user.type(screen.getByRole('textbox', { name: 'Header value, 1' }), 'v');
      await user.tab();
      await user.click(document.body);
      expect(await screen.findByText('Every entry needs a name.')).toBeInTheDocument();
    });
  });

  describe('one widget for each type of parameter', () => {
    it('draws and writes every type', async () => {
      const user = userEvent.setup();
      const { store } = renderWithStore(<NodeConfigForm nodeId="every" manifest={everyType} />, flowWithEveryNode());
      const writes = () => parametersOf(store.getState().document, 'every');

      // STRING
      await user.type(await screen.findByRole('textbox', { name: /Title/ }), 'Hello');
      expect(writes()['title']).toBe('Hello');
      // NUMBER: a number while the text is one, the raw text otherwise, nothing when empty.
      const limit = screen.getByRole('textbox', { name: 'Limit' });
      await user.type(limit, '12');
      expect(writes()['limit']).toBe(12);
      await user.type(limit, 'a');
      expect(writes()['limit']).toBe('12a');
      await user.tab();
      expect(await screen.findByText('Enter a number.')).toBeInTheDocument();
      await user.clear(limit);
      expect(writes()).not.toHaveProperty('limit');
      // BOOLEAN reveals the parameter that depends on it.
      expect(screen.queryByRole('textbox', { name: 'Extra' })).not.toBeInTheDocument();
      await user.click(screen.getByRole('switch', { name: 'Enabled' }));
      expect(writes()['enabled']).toBe(true);
      expect(await screen.findByRole('textbox', { name: 'Extra' })).toBeInTheDocument();
      // OPTIONS shows the label of the default.
      expect(screen.getByRole('combobox', { name: 'Mode' })).toHaveTextContent('Mode A');
      // MULTI_OPTIONS keeps the order of the options, not the order of the clicks.
      await user.click(screen.getByRole('checkbox', { name: 'Blue' }));
      await user.click(screen.getByRole('checkbox', { name: 'Red' }));
      expect(writes()['tags']).toEqual(['red', 'blue']);
      await user.click(screen.getByRole('checkbox', { name: 'Red' }));
      await user.click(screen.getByRole('checkbox', { name: 'Blue' }));
      expect(writes()).not.toHaveProperty('tags');
      // FIXED_COLLECTION
      await user.click(screen.getByRole('button', { name: 'Add to Pairs' }));
      await user.type(screen.getByRole('textbox', { name: 'Pair name, 1' }), 'k');
      expect(writes()['pairs']).toEqual([{ name: 'k', value: '' }]);
      // JSON is checked as JSON.
      const payload = screen.getByRole('textbox', { name: 'Payload' });
      await user.click(payload);
      await user.paste('{oops');
      await user.tab();
      expect(await screen.findByText('This is not valid JSON.')).toBeInTheDocument();
      await user.clear(payload);
      await user.click(payload);
      await user.paste('{"ok":true}');
      await waitFor(() => {
        expect(screen.queryByText('This is not valid JSON.')).not.toBeInTheDocument();
      });
      expect(writes()['payload']).toBe('{"ok":true}');
      // RESOURCE_LOCATOR: by identifier or by address.
      await user.type(within(screen.getByText('Target').closest('div') ?? document.body).getByRole('textbox'), 'abc');
      expect(writes()['target']).toEqual({ mode: 'id', value: 'abc' });
      await user.click(screen.getByRole('button', { name: 'By address' }));
      expect(writes()['target']).toEqual({ mode: 'url', value: 'abc' });
      // BINARY_PROPERTY shows its default and takes a name.
      expect(screen.getByRole('textbox', { name: 'File property' })).toHaveValue('data');
    });

    it('writes a date and time as an instant in UTC', async () => {
      const user = userEvent.setup();
      const { store } = renderWithStore(<NodeConfigForm nodeId="every" manifest={everyType} />, flowWithEveryNode());
      const when = await screen.findByLabelText('When');
      await user.type(when, '2026-10-14T09:30');
      const saved = parametersOf(store.getState().document, 'every')['when'];
      expect(typeof saved).toBe('string');
      expect(saved).toMatch(/^2026-10-14T\d\d:30:00Z$/);
    });
  });
});
