// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { CanvasNode, type GraphNode } from '@orcon/ui';
import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { describe, expect, it, test } from 'vitest';

import { renderInApp, server } from '../../../../test/screen-harness';
import { PublishControls } from '../forms/PublishControls';
import { exampleFlowDocument, EXAMPLE_FLOW_ID } from '../model/example-flow';
import { neverSendsTwice } from '../model/effect-safety';
import { createEditorStore } from '../store/editor-store';
import { EditorStoreProvider } from '../store/editor-context';

import { EditorTopBar } from './EditorTopBar';

const flowPath = `/api/v1/flows/${EXAMPLE_FLOW_ID}`;

function flowAnswer(state: { activationState: 'ACTIVE' | 'INACTIVE'; activeVersion: number | null; hasUnpublishedChanges: boolean }) {
  return HttpResponse.json(
    { id: EXAMPLE_FLOW_ID, projectId: '0195f1a0-0000-7000-8000-0000000000d1', name: 'Order notifications', draftRevision: 5, createdAt: '2026-10-10T09:00:00Z', updatedAt: '2026-10-13T16:00:00Z', ...state },
    { headers: { ETag: '"5"' } },
  );
}

/** The version that is published: the sample flow without its last branch and with another condition. */
function publishedVersion() {
  const older = exampleFlowDocument();
  older.nodes = older.nodes.filter((node) => node.id !== 'rejected');
  older.edges = older.edges.filter((edge) => edge.to.node !== 'rejected');
  const check = older.nodes.find((node) => node.id === 'validate');
  if (check) check.parameters = { condition: '={{ $json.total > 10 }}' };
  return HttpResponse.json({ flowId: EXAMPLE_FLOW_ID, version: 1, contentHash: 'sha256:abc', publishedAt: '2026-10-12T10:00:00Z', document: older });
}

function renderBar(status: 'saved' | 'unsaved' = 'saved') {
  // Every flow with a published version reads it to count the changes; a test that does not care gets the usual one.
  server.use(http.get(`${flowPath}/versions/1`, () => publishedVersion()));
  const store = createEditorStore({ document: exampleFlowDocument(), revision: '1' });
  return renderInApp(
    <EditorStoreProvider store={store}>
      <EditorTopBar
        editable
        status={status}
        flowId={EXAMPLE_FLOW_ID}
        nav={{
          projectLabel: 'Ventas',
          link: (target, { className, children }) => (
            <a href={target === 'project' ? '/flows' : '/runs'} className={className}>
              {children}
            </a>
          ),
        }}
      />
    </EditorStoreProvider>,
  );
}

describe('EditorTopBar', () => {
  it('shows Active and Publish together', async () => {
    server.use(http.get(flowPath, () => flowAnswer({ activationState: 'ACTIVE', activeVersion: 1, hasUnpublishedChanges: false })));
    renderBar();
    expect(await screen.findByRole('switch', { name: 'Active: Order notifications' })).toBeVisible();
    expect(screen.getByRole('button', { name: 'Publish' })).toBeVisible();
    expect(screen.getByRole('heading', { level: 1, name: 'Order notifications' })).toBeVisible();
    expect(screen.getByRole('link', { name: 'Ventas' })).toBeVisible();
    expect(screen.getByRole('link', { name: 'Executions' })).toBeVisible();
  });

  it('publish shows the pending count', async () => {
    server.use(
      http.get(flowPath, () => flowAnswer({ activationState: 'ACTIVE', activeVersion: 1, hasUnpublishedChanges: true })),
      http.get(`${flowPath}/versions/1`, () => publishedVersion()),
    );
    renderBar();
    // One node, one connection and one condition differ from the published version.
    expect(await screen.findByRole('button', { name: 'Publish, 3 changes not published yet' })).toBeVisible();
  });

  it('active is disabled with a reason without a published version', async () => {
    server.use(http.get(flowPath, () => flowAnswer({ activationState: 'INACTIVE', activeVersion: null, hasUnpublishedChanges: true })));
    renderBar();
    const toggle = await screen.findByRole('switch', { name: 'Active: Order notifications' });
    expect(toggle).toHaveAttribute('aria-disabled', 'true');
    expect(screen.getByText('Publish a version first: only a published version can run.')).toBeInTheDocument();
    // No published version, so Publish carries no number.
    expect(screen.getByRole('button', { name: 'Publish' })).toBeVisible();
  });

  it('shows the saved text state', async () => {
    server.use(http.get(flowPath, () => flowAnswer({ activationState: 'INACTIVE', activeVersion: null, hasUnpublishedChanges: true })));
    const view = renderBar('saved');
    expect(await screen.findByText('Saved')).toBeVisible();
    view.unmount();
    renderBar('unsaved');
    expect(await screen.findByText('Unsaved changes')).toBeVisible();
  });

  it('hides tags and share', async () => {
    server.use(http.get(flowPath, () => flowAnswer({ activationState: 'ACTIVE', activeVersion: 1, hasUnpublishedChanges: false })));
    renderBar();
    await screen.findByRole('switch', { name: 'Active: Order notifications' });
    expect(screen.queryByText(/^(Share|Tags?|Add tag|Compartir|Etiqueta)/)).toBeNull();
    expect(screen.queryByRole('button', { name: /share|tag/i })).toBeNull();
  });

  it('renames the flow in place with Enter', async () => {
    server.use(http.get(flowPath, () => flowAnswer({ activationState: 'ACTIVE', activeVersion: 1, hasUnpublishedChanges: false })));
    renderBar();
    await userEvent.click(await screen.findByRole('button', { name: 'Order notifications' }));
    const field = screen.getByRole('textbox', { name: 'Flow name' });
    await userEvent.clear(field);
    await userEvent.type(field, 'Orders v2{Enter}');
    expect(await screen.findByRole('heading', { level: 1, name: 'Orders v2' })).toBeVisible();
  });
});

const node = (neverTwice: boolean): GraphNode => ({
  id: 'send',
  kind: 'action',
  name: 'Send message',
  inputs: [{ id: 'main' }],
  outputs: [{ id: 'main' }],
  position: { x: 0, y: 0 },
  neverTwice,
});

test('never sends twice badge appears only for idempotency key or reconcile effects', () => {
  expect(neverSendsTwice({ purity: 'EFFECT', safetyLevel: 'IDEMPOTENCY_KEY', key: 'message.wamid' })).toBe(true);
  expect(neverSendsTwice({ purity: 'EFFECT', safetyLevel: 'RECONCILE' })).toBe(true);
  // A step that only reads, or that changes nothing outside, makes no promise about repeating.
  expect(neverSendsTwice({ purity: 'READ', safetyLevel: 'SAFE' })).toBe(false);
  expect(neverSendsTwice({ purity: 'PURE', safetyLevel: 'SAFE' })).toBe(false);
  expect(neverSendsTwice(undefined)).toBe(false);

  const { unmount } = renderInApp(<CanvasNode node={node(true)} />);
  expect(screen.getByText('Never sends twice')).toBeVisible();
  unmount();
  renderInApp(<CanvasNode node={node(false)} />);
  expect(screen.queryByText('Never sends twice')).toBeNull();
});

test('never sends twice badge never appears for unsafe effects', () => {
  expect(neverSendsTwice({ purity: 'EFFECT', safetyLevel: 'UNSAFE' })).toBe(false);
  // The editor's own types send nothing to a service: none of them carries the badge.
  renderInApp(<CanvasNode node={node(neverSendsTwice({ purity: 'EFFECT', safetyLevel: 'UNSAFE' }))} />);
  expect(screen.queryByText('Never sends twice')).toBeNull();
});

test('the publish controls stay usable on their own', async () => {
  server.use(http.get(flowPath, () => flowAnswer({ activationState: 'ACTIVE', activeVersion: 1, hasUnpublishedChanges: false })));
  renderInApp(<PublishControls flowId={EXAMPLE_FLOW_ID} flowName="Order notifications" />);
  expect(await screen.findByRole('switch', { name: 'Active: Order notifications' })).toBeChecked();
});
