// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createMemoryHistory, createRootRouteWithContext, createRoute, createRouter, Outlet, RouterProvider } from '@tanstack/react-router';
import { act, fireEvent, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { renderInApp } from '../../../../test/screen-harness';
import { FIRST_USEFUL_MARK } from '../../../app/perf/marks';
import { createQueryClient } from '../../../app/query-client';
import type { RouterContext } from '../../../app/router-context';
import { createLocalDraftSource } from '../api/local-draft-source';
import { draftSource } from '../api/queries';
import { EXAMPLE_FLOW_ID, exampleFlowDocument } from '../model/example-flow';
import type { FlowDocument } from '../model/flow-document';

import { EditorScreen } from './EditorScreen';

/** The editor opens where the app opens it: `/flows/<id>`. The route here has no loader, so the screen itself shows every state. */
function renderEditor(path = `/flows/${EXAMPLE_FLOW_ID}`) {
  const queryClient = createQueryClient();
  const root = createRootRouteWithContext<RouterContext>()({ component: Outlet });
  const flow = createRoute({ getParentRoute: () => root, path: '/flows/$flowId', component: EditorScreen });
  const router = createRouter({ routeTree: root.addChildren([flow]), history: createMemoryHistory({ initialEntries: [path] }), context: { queryClient } });
  return renderInApp(<RouterProvider router={router} />, { queryClient });
}

/** A tablet or a computer: from the md breakpoint the canvas edits. */
function editing(on = true) {
  vi.stubGlobal(
    'matchMedia',
    vi.fn((query: string) => ({ matches: on && query === '(min-width: 48rem)', media: query, addEventListener: vi.fn(), removeEventListener: vi.fn() })),
  );
}

const sample = (change?: (document: FlowDocument) => void): FlowDocument => {
  const document = exampleFlowDocument();
  change?.(document);
  return document;
};

function serve(document: FlowDocument, revision = '1') {
  vi.spyOn(draftSource, 'load').mockResolvedValue({ document, revision });
}

/**
 * A node of the canvas by the label it announces. jsdom never measures a node, so the canvas keeps its tiles
 * `visibility: hidden` and the accessibility tree gives them no name: they are found by their label instead.
 */
function canvasNode(label: RegExp): HTMLElement {
  const found = screen.getAllByRole('group', { hidden: true }).filter((element) => label.test(element.getAttribute('aria-label') ?? ''));
  if (found.length !== 1 || !found[0]) throw new Error(`Expected one node labelled ${String(label)}, found ${String(found.length)}.`);
  return found[0];
}

/** The editor is open when its heading is shown and the canvas has drawn the nodes (it does so after the first render). */
async function opened() {
  await screen.findByRole('heading', { level: 1, name: 'Order notifications' });
  await waitFor(() => {
    canvasNode(/^Slack,/);
  });
}

const status = () => screen.getAllByRole('status').map((element) => element.textContent).join(' | ');

describe('the editor screen', () => {
  beforeEach(() => {
    editing();
    // Every test starts from the sample flow at revision 1: the app's source keeps what an earlier test saved.
    const fresh = createLocalDraftSource();
    vi.spyOn(draftSource, 'load').mockImplementation(fresh.load);
    vi.spyOn(draftSource, 'save').mockImplementation(fresh.save);
  });
  afterEach(() => {
    vi.restoreAllMocks();
    vi.useRealTimers();
  });

  describe('its five states', () => {
    it('shows placeholders while the flow loads', async () => {
      vi.spyOn(draftSource, 'load').mockReturnValue(new Promise<never>(() => undefined));
      renderEditor();
      expect(await screen.findByRole('status', { name: 'Loading' })).toHaveAttribute('aria-busy', 'true');
    });

    it('says that a flow with no nodes is empty and offers the first step', async () => {
      serve(sample((document) => Object.assign(document, { nodes: [], edges: [], editor: {} })));
      renderEditor();
      expect(await screen.findByRole('heading', { level: 1, name: 'Order notifications' })).toBeVisible();
      expect(screen.getByRole('button', { name: /Add first step/ })).toBeVisible();
      expect(screen.getByRole('region', { name: 'Flow canvas: 0 nodes' })).toBeVisible();
    });

    it('explains an error and tries again', async () => {
      const load = vi.spyOn(draftSource, 'load').mockRejectedValueOnce({ status: 500, title: 'The server could not read the flow' });
      load.mockResolvedValue({ document: sample(), revision: '1' });
      renderEditor();
      expect(await screen.findByRole('button', { name: 'Try again' })).toBeVisible();
      await userEvent.click(screen.getByRole('button', { name: 'Try again' }));
      expect(await screen.findByRole('heading', { level: 1, name: 'Order notifications' })).toBeVisible();
    });

    it('says a person without permission has none, and offers no retry', async () => {
      vi.spyOn(draftSource, 'load').mockRejectedValue({ status: 403, title: 'Forbidden' });
      renderEditor();
      expect(await screen.findByText('You do not have permission to see this')).toBeVisible();
      expect(screen.queryByRole('button', { name: 'Try again' })).toBeNull();
    });

    it('shows the flow: its name, the sample notice, the tools and the canvas with every node', async () => {
      const mark = vi.spyOn(performance, 'mark');
      renderEditor();
      await opened();
      expect(screen.getByRole('heading', { level: 1, name: 'Order notifications' })).toBeVisible();
      expect(screen.getByText('Preview with a sample flow')).toBeVisible();
      expect(screen.getByRole('toolbar', { name: 'Flow editor tools' })).toBeVisible();
      expect(screen.getByRole('region', { name: 'Flow canvas: 6 nodes' })).toBeVisible();
      for (const name of [/^Webhook,/, /^Validate order,/, /^HTTP Request,/, /^Slack,/, /^Ignore invalid order,/, /^Done,/]) expect(canvasNode(name)).toBeInTheDocument();
      expect(status()).toContain('Saved');
      expect(mark).toHaveBeenCalledWith(FIRST_USEFUL_MARK, { detail: { route: '/flows/$flowId' } });
    });
  });

  describe('on a phone', () => {
    it('shows the flow read-only: no editing tool, no save state, no sample notice', async () => {
      editing(false);
      renderEditor();
      expect(await screen.findByRole('heading', { level: 1, name: 'Order notifications' })).toBeVisible();
      expect(screen.getByRole('button', { name: 'Flow outline' })).toBeVisible();
      for (const name of ['Add node', 'Add sticky note', 'Undo', 'Redo', 'Tidy up', 'Find a node', 'Commands', 'Save now', 'Keyboard shortcuts']) {
        expect(screen.queryByRole('button', { name })).toBeNull();
      }
      expect(screen.queryByText('Preview with a sample flow')).toBeNull();
      expect(screen.queryByText('Saved')).toBeNull();
      expect(screen.getByText(/View only/)).toBeVisible();
    });

    it('ignores the keyboard shortcuts of the editor', async () => {
      editing(false);
      renderEditor();
      await opened();
      const node = canvasNode(/^Slack,/);
      node.focus();
      await userEvent.keyboard('{Control>}f{/Control}');
      expect(screen.queryByRole('dialog')).toBeNull();
    });
  });

  describe('the tools', () => {
    it('opens the keyboard shortcuts from the button and from ?, listed from the keymap', async () => {
      renderEditor();
      await opened();
      await userEvent.click(screen.getByRole('button', { name: 'Keyboard shortcuts' }));
      const dialog = await screen.findByRole('dialog', { name: 'Keyboard shortcuts' });
      expect(within(dialog).getByText('Tidy up the selection, or the whole flow')).toBeVisible();
      expect(within(dialog).queryByText('Pin or unpin the output of the focused node')).toBeNull();
      await userEvent.keyboard('{Escape}');
      await waitFor(() => {
        expect(screen.queryByRole('dialog')).toBeNull();
      });
      canvasNode(/^Slack,/).focus();
      await userEvent.keyboard('?');
      expect(await screen.findByRole('dialog', { name: 'Keyboard shortcuts' })).toBeVisible();
    });

    it('finds a node by name and moves focus to it', async () => {
      renderEditor();
      await opened();
      canvasNode(/^Webhook,/).focus();
      await userEvent.keyboard('{Control>}f{/Control}');
      const dialog = await screen.findByRole('dialog', { name: 'Find a node' });
      await userEvent.type(within(dialog).getByRole('textbox', { name: 'Node name' }), 'sla');
      expect(within(dialog).getAllByRole('button', { name: /Slack/ })).toHaveLength(1);
      await userEvent.keyboard('{Enter}');
      await waitFor(() => {
        expect(canvasNode(/^Slack,/)).toHaveFocus();
      });
      await userEvent.keyboard('{Control>}f{/Control}');
      await userEvent.type(await screen.findByRole('textbox', { name: 'Node name' }), 'nothing like this');
      expect(await screen.findByText('No node has that name.')).toBeVisible();
    });

    it('lists every command in the palette and runs one', async () => {
      renderEditor();
      await opened();
      await userEvent.click(screen.getByRole('button', { name: 'Commands' }));
      const dialog = await screen.findByRole('dialog', { name: 'Command palette' });
      expect(within(dialog).getByRole('option', { name: /Zoom to fit/ })).toBeVisible();
      await userEvent.click(within(dialog).getByRole('option', { name: /Keyboard shortcuts/ }));
      expect(await screen.findByRole('dialog', { name: 'Keyboard shortcuts' })).toBeVisible();
    });

    it('shows and hides the outline, with the notes by their title and the steps that cannot run', async () => {
      serve(sample((document) => document.nodes.push({ id: 'old', name: 'Old node', element: 'task', type: 'orcon.unsupported@1', parameters: {} })));
      renderEditor();
      await opened();
      const toggle = screen.getByRole('button', { name: 'Flow outline' });
      expect(toggle).toHaveAttribute('aria-pressed', 'false');
      await userEvent.click(toggle);
      const outline = screen.getByRole('complementary', { name: 'Flow outline' });
      expect(within(outline).getByText('Steps that need attention')).toBeVisible();
      expect(within(outline).getByText('Old node: not supported, it cannot run and blocks publishing')).toBeVisible();
      expect(within(outline).getByText('Note: Order intake')).toBeVisible();
      expect(within(outline).getAllByText('Disabled').length).toBeGreaterThan(0);
      await userEvent.click(within(outline).getByRole('button', { name: 'Delete Old node' }));
      await waitFor(() => {
        expect(within(screen.getByRole('complementary', { name: 'Flow outline' })).queryByText('Steps that need attention')).toBeNull();
      });
      await userEvent.click(toggle);
      expect(screen.queryByRole('complementary', { name: 'Flow outline' })).toBeNull();
    });

    it('adds a node from the palette of the canvas', async () => {
      renderEditor();
      await opened();
      await userEvent.click(screen.getByRole('button', { name: 'Add node' }));
      const panel = await screen.findByRole('complementary', { name: 'Add a node' });
      await userEvent.click(within(panel).getByRole('option', { name: /Edit fields/ }));
      expect(await screen.findByRole('region', { name: 'Flow canvas: 7 nodes' })).toBeVisible();
      expect(canvasNode(/^Edit fields,/)).toBeInTheDocument();
    });

    it('gives a new node a free name when its name is taken, and the canvas shows it', async () => {
      renderEditor();
      await opened();
      await userEvent.click(screen.getByRole('button', { name: 'Add node' }));
      const panel = await screen.findByRole('complementary', { name: 'Add a node' });
      await userEvent.click(within(panel).getByRole('option', { name: /^HTTP Request/ }));
      await waitFor(() => {
        expect(canvasNode(/^HTTP Request 2,/)).toBeInTheDocument();
      });
      // One user action, one history entry: the rename that follows the add is not a second step.
      await userEvent.click(screen.getByRole('button', { name: 'Undo' }));
      expect(await screen.findByRole('region', { name: 'Flow canvas: 6 nodes' })).toBeVisible();
      expect(screen.getByRole('button', { name: 'Undo' })).toBeDisabled();
    });
  });

  describe('editing and history', () => {
    it('follows what the canvas does, undoes and redoes it with the keyboard and the buttons', async () => {
      renderEditor();
      await opened();
      expect(screen.getByRole('button', { name: 'Undo' })).toBeDisabled();
      expect(screen.getByRole('button', { name: 'Redo' })).toBeDisabled();

      const slack = canvasNode(/^Slack,/);
      slack.focus();
      await userEvent.keyboard('d');
      await waitFor(() => {
        expect(canvasNode(/^Slack,/).getAttribute('aria-label')).toContain('disabled');
      });
      expect(screen.getByRole('button', { name: 'Undo' })).toBeEnabled();
      expect(status()).toContain('Unsaved changes');

      await userEvent.keyboard('{Control>}z{/Control}');
      await waitFor(() => {
        expect(canvasNode(/^Slack,/).getAttribute('aria-label')).not.toContain('disabled');
      });
      expect(screen.getByRole('button', { name: 'Undo' })).toBeDisabled();
      expect(screen.getByRole('button', { name: 'Redo' })).toBeEnabled();

      await userEvent.keyboard('{Control>}{Shift>}z{/Shift}{/Control}');
      await waitFor(() => {
        expect(canvasNode(/^Slack,/).getAttribute('aria-label')).toContain('disabled');
      });

      await userEvent.click(screen.getByRole('button', { name: 'Undo' }));
      await waitFor(() => {
        expect(canvasNode(/^Slack,/).getAttribute('aria-label')).not.toContain('disabled');
      });
      await userEvent.click(screen.getByRole('button', { name: 'Redo' }));
      await waitFor(() => {
        expect(canvasNode(/^Slack,/).getAttribute('aria-label')).toContain('disabled');
      });
    });

    it('says there is nothing to undo when nothing was done', async () => {
      renderEditor();
      await opened();
      canvasNode(/^Slack,/).focus();
      await userEvent.keyboard('{Control>}z{/Control}');
      expect(await screen.findByText('Nothing to undo')).toBeVisible();
    });

    it('refuses to pin and to run, and says why, without changing the flow', async () => {
      renderEditor();
      await opened();
      canvasNode(/^Slack,/).focus();
      await userEvent.keyboard('p');
      expect(await screen.findByText('Pinning data arrives in a later editor release.')).toBeVisible();
      await userEvent.keyboard('r');
      expect(await screen.findByText('Test runs arrive in a later editor release.')).toBeVisible();
      expect(screen.getByRole('button', { name: 'Undo' })).toBeDisabled();
      await userEvent.keyboard('{Control>}{Enter}{/Control}');
      expect(screen.getByText('Test runs arrive in a later editor release.')).toBeVisible();
    });

    it('tidies up the flow with the button and the keyboard, and the change can be undone', async () => {
      renderEditor();
      await opened();
      await userEvent.click(screen.getByRole('button', { name: 'Tidy up' }));
      // The first tidy up loads the layout library, which can take a moment.
      expect(await screen.findByText('Tidied up 7 items', {}, { timeout: 5_000 })).toBeVisible();
      expect(screen.getByRole('button', { name: 'Undo' })).toBeEnabled();
      await userEvent.click(screen.getByRole('button', { name: 'Undo' }));
      await waitFor(() => {
        expect(screen.getByRole('button', { name: 'Undo' })).toBeDisabled();
      });
      canvasNode(/^Slack,/).focus();
      await userEvent.keyboard('{Shift>}{Alt>}t{/Alt}{/Shift}');
      await waitFor(() => {
        expect(screen.getByRole('button', { name: 'Undo' })).toBeEnabled();
      });
    });
  });

  describe('the clipboard', () => {
    const clipboard = () => {
      const data = new Map<string, string>();
      return { setData: (type: string, value: string) => data.set(type, value), getData: (type: string) => data.get(type) ?? '', data };
    };

    async function select(...names: RegExp[]) {
      for (const name of names) {
        canvasNode(name).focus();
        await userEvent.keyboard(' ');
      }
    }

    it('copies the selected nodes as an Orcon fragment and pastes them with new names', async () => {
      renderEditor();
      await opened();
      await select(/^HTTP Request,/, /^Slack,/);
      const board = clipboard();
      fireEvent.copy(canvasNode(/^Slack,/), { clipboardData: board });
      const text = board.data.get('text/plain') ?? '';
      expect(JSON.parse(text)).toMatchObject({ format: 'orcon/flow-fragment@1', nodes: [{ id: 'inventory' }, { id: 'slack' }] });
      expect(await screen.findByText('Copied 2 items')).toBeVisible();

      fireEvent.paste(canvasNode(/^Slack,/), { clipboardData: board });
      expect(await screen.findByText('Pasted 2 items')).toBeVisible();
      expect(await screen.findByRole('region', { name: 'Flow canvas: 8 nodes' })).toBeVisible();
      expect(canvasNode(/^HTTP Request 2,/)).toBeInTheDocument();
      expect(canvasNode(/^Slack 2,/)).toBeInTheDocument();
    });

    it('cuts: it copies and removes the selection, and the cut can be undone', async () => {
      renderEditor();
      await opened();
      await select(/^Done,/);
      const board = clipboard();
      fireEvent.cut(canvasNode(/^Done,/), { clipboardData: board });
      expect(await screen.findByRole('region', { name: 'Flow canvas: 5 nodes' })).toBeVisible();
      expect(board.data.get('text/plain')).toContain('orcon/flow-fragment@1');
      await userEvent.click(screen.getByRole('button', { name: 'Undo' }));
      expect(await screen.findByRole('region', { name: 'Flow canvas: 6 nodes' })).toBeVisible();
    });

    it('duplicates the selection with the keyboard', async () => {
      renderEditor();
      await opened();
      await select(/^Slack,/);
      await userEvent.keyboard('{Control>}d{/Control}');
      expect(await screen.findByRole('region', { name: 'Flow canvas: 7 nodes' })).toBeVisible();
      expect(canvasNode(/^Slack 2,/)).toBeInTheDocument();
    });

    it('asks for a selection first', async () => {
      renderEditor();
      await opened();
      canvasNode(/^Slack,/).focus();
      await userEvent.keyboard('{Control>}d{/Control}');
      expect(await screen.findByText('Select at least one node first')).toBeVisible();
      fireEvent.copy(canvasNode(/^Slack,/), { clipboardData: clipboard() });
      expect(screen.getByText('Select at least one node first')).toBeVisible();
    });

    it.each([
      ['not nodes', 'just some words', 'The clipboard does not contain nodes'],
      ['nothing', '', 'The clipboard does not contain nodes'],
      ['an n8n workflow', JSON.stringify({ nodes: [{ type: 'n8n-nodes-base.webhook' }], connections: {} }), 'This looks like an n8n workflow. It is converted on the server, which this preview does not reach yet.'],
      ['a fragment that is not valid', JSON.stringify({ format: 'orcon/flow-fragment@1', nodes: [{ id: 'bad id' }], edges: [], editor: {} }), 'The pasted nodes are not valid, so nothing was added.'],
      ['too much text', 'x'.repeat(2 * 1024 * 1024 + 1), 'That is too large to paste. The limit is 2 MiB.'],
    ])('refuses to paste %s, says why and adds nothing', async (_label, text, message) => {
      renderEditor();
      await opened();
      fireEvent.paste(canvasNode(/^Slack,/), { clipboardData: { getData: () => text, setData: vi.fn() } });
      expect(await screen.findByText(message)).toBeVisible();
      expect(screen.getByRole('region', { name: 'Flow canvas: 6 nodes' })).toBeVisible();
    });

    it('pastes a hand-written fragment under the rules of a connection and says what it left out', async () => {
      renderEditor();
      await opened();
      const node = (id: string, type: string) => ({ id, name: id, element: 'task', type, parameters: {} });
      const text = JSON.stringify({
        format: 'orcon/flow-fragment@1',
        nodes: [node('a', 'orcon.http@1'), node('b', 'orcon.http@1'), node('x', 'evil.unknown@9')],
        edges: [
          { id: '1', from: { node: 'a', port: 'main' }, to: { node: 'b' }, kind: 'SEQUENCE' },
          { id: '2', from: { node: 'b', port: 'main' }, to: { node: 'a' }, kind: 'SEQUENCE' },
        ],
        editor: { nodes: { a: { x: 1e9, y: 0 } } },
      });
      fireEvent.paste(canvasNode(/^Slack,/), { clipboardData: { getData: () => text, setData: vi.fn() } });
      expect(
        await screen.findByText(
          'Pasted 3 items. 1 connection was not pasted because it breaks the connection rules. 1 node has a type this editor does not know: it was added disabled and cannot run',
        ),
      ).toBeVisible();
    });

    it('leaves copy, cut and paste alone inside a text field', async () => {
      renderEditor();
      await opened();
      await userEvent.click(screen.getByRole('button', { name: 'Find a node' }));
      const field = await screen.findByRole('textbox', { name: 'Node name' });
      const board = clipboard();
      fireEvent.copy(field, { clipboardData: board });
      expect(board.data.size).toBe(0);
    });
  });

  describe('saving', () => {
    it('saves two seconds after a change, with the revision it started from, and says so', async () => {
      const save = vi.spyOn(draftSource, 'save');
      renderEditor();
      await opened();
      vi.useFakeTimers({ shouldAdvanceTime: true });
      canvasNode(/^Slack,/).focus();
      fireEvent.keyDown(canvasNode(/^Slack,/), { key: 'd' });
      await waitFor(() => {
        expect(status()).toContain('Unsaved changes');
      });
      await act(async () => {
        await vi.advanceTimersByTimeAsync(2_100);
      });
      await waitFor(() => {
        expect(status()).toContain('Saved');
      });
      expect(save).toHaveBeenCalledTimes(1);
      expect(save).toHaveBeenCalledWith(EXAMPLE_FLOW_ID, expect.objectContaining({ name: 'Order notifications' }), '1');
    });

    it('saves now with the button and with Ctrl+S', async () => {
      const save = vi.spyOn(draftSource, 'save');
      renderEditor();
      await opened();
      canvasNode(/^Slack,/).focus();
      await userEvent.keyboard('d');
      await userEvent.click(screen.getByRole('button', { name: 'Save now' }));
      await waitFor(() => {
        expect(save).toHaveBeenCalledTimes(1);
      });
      canvasNode(/^Slack,/).focus();
      await userEvent.keyboard('d');
      await userEvent.keyboard('{Control>}s{/Control}');
      await waitFor(() => {
        expect(save).toHaveBeenCalledTimes(2);
      });
      expect(save).toHaveBeenLastCalledWith(EXAMPLE_FLOW_ID, expect.anything(), '2');
    });

    it('keeps the working copy and tries again when the save fails', async () => {
      const save = vi.spyOn(draftSource, 'save').mockRejectedValueOnce(new TypeError('Failed to fetch'));
      renderEditor();
      await opened();
      canvasNode(/^Slack,/).focus();
      await userEvent.keyboard('d');
      await userEvent.click(screen.getByRole('button', { name: 'Save now' }));
      expect(await screen.findByText('Not saved, trying again')).toBeVisible();
      expect(canvasNode(/^Slack,/).getAttribute('aria-label')).toContain('disabled');
      expect(save).toHaveBeenCalledTimes(1);
    });

    it('never overwrites: a conflict stops saving and offers to reload, after a confirmation', async () => {
      const save = vi.spyOn(draftSource, 'save').mockResolvedValue({ kind: 'conflict' });
      const newer: FlowDocument = sample((document) => {
        document.name = 'Order notifications, edited elsewhere';
      });
      renderEditor();
      await opened();
      canvasNode(/^Slack,/).focus();
      await userEvent.keyboard('d');
      await userEvent.click(screen.getByRole('button', { name: 'Save now' }));
      expect(await screen.findByText('This flow changed somewhere else')).toBeVisible();
      expect(screen.getAllByText('Not saved, the flow changed elsewhere').length).toBeGreaterThan(0);
      await userEvent.keyboard('d');
      await userEvent.click(screen.getByRole('button', { name: 'Save now' }));
      expect(save).toHaveBeenCalledTimes(1);

      vi.spyOn(draftSource, 'load').mockResolvedValue({ document: newer, revision: '5' });
      await userEvent.click(screen.getByRole('button', { name: 'Reload the flow' }));
      const confirm = await screen.findByRole('alertdialog');
      expect(within(confirm).getByText('The changes you made in this tab since the last save are lost.')).toBeVisible();
      await userEvent.click(within(confirm).getByRole('button', { name: 'Discard and reload' }));
      expect(await screen.findByRole('heading', { level: 1, name: 'Order notifications, edited elsewhere' })).toBeVisible();
      await waitFor(() => {
        expect(screen.queryByText('This flow changed somewhere else')).toBeNull();
      });
      expect(screen.getByText('Saved')).toBeVisible();
    });
  });

  it('warns the browser when somebody leaves with changes that are not saved', async () => {
    renderEditor();
    await opened();
    const leave = () => {
      const event = new Event('beforeunload', { cancelable: true });
      window.dispatchEvent(event);
      return event.defaultPrevented;
    };
    expect(leave()).toBe(false);
    canvasNode(/^Slack,/).focus();
    await userEvent.keyboard('d');
    expect(leave()).toBe(true);
  });
});
