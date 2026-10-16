// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { describe, expect, it } from 'vitest';

import { problem } from '../../../../test/session-fixtures';
import { renderInApp, server } from '../../../../test/screen-harness';
import { EXAMPLE_FLOW_ID } from '../model/example-flow';

import { PublishControls } from './PublishControls';

const flowPath = `/api/v1/flows/${EXAMPLE_FLOW_ID}`;

/** A flow as `getFlow` answers it, with the entity tag of the representation. */
function flowAnswer(state: { activationState: 'ACTIVE' | 'INACTIVE'; activeVersion: number | null; hasUnpublishedChanges: boolean }, etag: string) {
  return HttpResponse.json(
    { id: EXAMPLE_FLOW_ID, projectId: '0195f1a0-0000-7000-8000-0000000000d1', name: 'Order notifications', draftRevision: 5, createdAt: '2026-10-10T09:00:00Z', updatedAt: '2026-10-13T16:00:00Z', ...state },
    { headers: { ETag: etag } },
  );
}

describe('PublishControls', () => {
  it('shows the activation state as a switch with its word and the pending changes on Publish', async () => {
    server.use(http.get(flowPath, () => flowAnswer({ activationState: 'ACTIVE', activeVersion: 2, hasUnpublishedChanges: true }, '"5"')));
    renderInApp(<PublishControls flowId={EXAMPLE_FLOW_ID} flowName="Order notifications" pending={3} />);
    expect(await screen.findByText('Active')).toBeVisible();
    expect(screen.getByRole('switch', { name: 'Active: Order notifications' })).toBeChecked();
    expect(screen.getByRole('button', { name: 'Publish, 3 changes not published yet' })).toBeVisible();
  });

  it('activates a published flow against the entity tag it read', async () => {
    let ifMatch: string | null = null;
    let activated = false;
    server.use(
      http.get(flowPath, () => (activated ? flowAnswer({ activationState: 'ACTIVE', activeVersion: 1, hasUnpublishedChanges: false }, '"6"') : flowAnswer({ activationState: 'INACTIVE', activeVersion: 1, hasUnpublishedChanges: false }, '"5"'))),
      http.post(`${flowPath}/activate`, ({ request }) => {
        ifMatch = request.headers.get('If-Match');
        activated = true;
        return flowAnswer({ activationState: 'ACTIVE', activeVersion: 1, hasUnpublishedChanges: false }, '"6"');
      }),
    );
    renderInApp(<PublishControls flowId={EXAMPLE_FLOW_ID} flowName="Order notifications" />);
    await userEvent.click(await screen.findByRole('switch', { name: 'Active: Order notifications' }));
    await waitFor(() => {
      expect(screen.getByRole('switch', { name: 'Active: Order notifications' })).toBeChecked();
    });
    expect(ifMatch).toBe('"5"');
  });

  it('refuses an activation made on a flow that changed, reads the flow again and changes nothing', async () => {
    let reads = 0;
    server.use(
      http.get(flowPath, () => {
        reads += 1;
        return flowAnswer({ activationState: 'INACTIVE', activeVersion: 1, hasUnpublishedChanges: false }, reads === 1 ? '"5"' : '"7"');
      }),
      http.post(`${flowPath}/activate`, () => problem(412, 'precondition-failed', 'precondition-failed')),
    );
    renderInApp(<PublishControls flowId={EXAMPLE_FLOW_ID} flowName="Order notifications" />);
    await userEvent.click(await screen.findByRole('switch', { name: 'Active: Order notifications' }));
    expect(await screen.findByText('Nothing was changed. The flow changed somewhere else and now shows its newest state.')).toBeVisible();
    await waitFor(() => {
      expect(reads).toBe(2);
    });
    expect(screen.getByText('Inactive')).toBeVisible();
  });

  it('explains that a flow must be published first when the server answers 409', async () => {
    server.use(
      http.get(flowPath, () => flowAnswer({ activationState: 'INACTIVE', activeVersion: 1, hasUnpublishedChanges: false }, '"5"')),
      http.post(`${flowPath}/activate`, () => problem(409, 'conflict', 'conflict')),
    );
    renderInApp(<PublishControls flowId={EXAMPLE_FLOW_ID} flowName="Order notifications" />);
    await userEvent.click(await screen.findByRole('switch', { name: 'Active: Order notifications' }));
    expect(await screen.findByText('Publish a version first: only a published version can run.')).toBeVisible();
    expect(screen.getByText('Inactive')).toBeVisible();
  });

  describe('publishing', () => {
    const draftAnswer = () =>
      HttpResponse.json(
        { flowId: EXAMPLE_FLOW_ID, revision: 3, document: { format: 'orcon/flow@1', id: 'flow', name: 'Order notifications', settings: {}, variables: [], nodes: [], edges: [], editor: {} }, updatedAt: '2026-10-13T16:00:00Z' },
        { headers: { ETag: '"3"' } },
      );

    it('sends the entity tag of the draft, not the one of the flow, and reads both again', async () => {
      let ifMatch: string | null = null;
      let flowReads = 0;
      let draftReads = 0;
      server.use(
        http.get(flowPath, () => {
          flowReads += 1;
          return flowAnswer({ activationState: 'INACTIVE', activeVersion: null, hasUnpublishedChanges: true }, '"9"');
        }),
        http.get(`${flowPath}/draft`, () => {
          draftReads += 1;
          return draftAnswer();
        }),
        http.post(`${flowPath}/publish`, ({ request }) => {
          ifMatch = request.headers.get('If-Match');
          return HttpResponse.json({ flowId: EXAMPLE_FLOW_ID, version: 1, contentHash: 'sha256:abc', publishedAt: '2026-10-13T16:00:00Z' });
        }),
      );
      renderInApp(<PublishControls flowId={EXAMPLE_FLOW_ID} flowName="Order notifications" />);
      await userEvent.click(await screen.findByRole('button', { name: 'Publish' }));
      const dialog = await screen.findByRole('dialog');
      await userEvent.click(within(dialog).getByRole('button', { name: 'Publish' }));
      await waitFor(() => {
        expect(ifMatch).toBe('"3"');
      });
      await waitFor(() => {
        expect(flowReads).toBeGreaterThanOrEqual(2);
      });
      expect(draftReads).toBeGreaterThanOrEqual(1);
    });

    it('refuses a draft that moved on, publishes nothing and says so', async () => {
      server.use(
        http.get(flowPath, () => flowAnswer({ activationState: 'INACTIVE', activeVersion: null, hasUnpublishedChanges: true }, '"9"')),
        http.get(`${flowPath}/draft`, () => draftAnswer()),
        http.post(`${flowPath}/publish`, () => problem(412, 'precondition-failed', 'precondition-failed')),
      );
      renderInApp(<PublishControls flowId={EXAMPLE_FLOW_ID} flowName="Order notifications" />);
      await userEvent.click(await screen.findByRole('button', { name: 'Publish' }));
      const dialog = await screen.findByRole('dialog');
      await userEvent.click(within(dialog).getByRole('button', { name: 'Publish' }));
      expect(await within(dialog).findByText('Nothing was published. The draft changed somewhere else and was read again; check it, then publish.')).toBeVisible();
    });
  });
});
