// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen, waitFor } from '@testing-library/react';
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
    server.use(
      http.get(flowPath, () => flowAnswer({ activationState: 'INACTIVE', activeVersion: 1, hasUnpublishedChanges: false }, '"5"')),
      http.post(`${flowPath}/activate`, ({ request }) => {
        ifMatch = request.headers.get('If-Match');
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

  it('refuses an activation made on a flow that changed, changes nothing and offers a reload', async () => {
    server.use(
      http.get(flowPath, () => flowAnswer({ activationState: 'INACTIVE', activeVersion: 1, hasUnpublishedChanges: false }, '"5"')),
      http.post(`${flowPath}/activate`, () => problem(412, 'precondition-failed', 'precondition-failed')),
    );
    renderInApp(<PublishControls flowId={EXAMPLE_FLOW_ID} flowName="Order notifications" />);
    await userEvent.click(await screen.findByRole('switch', { name: 'Active: Order notifications' }));
    expect(await screen.findByText('Nothing was changed. Reload the flow to see its newest state.')).toBeVisible();
    expect(screen.getByRole('button', { name: 'Reload the flow' })).toBeVisible();
    expect(screen.getByText('Inactive')).toBeVisible();
  });
});
