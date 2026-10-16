// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { renderInApp, server } from '../../../../test/screen-harness';
import { EXAMPLE_FLOW_ID } from '../model/example-flow';

import { PublishDialog } from './PublishDialog';

const publishPath = `/api/v1/flows/${EXAMPLE_FLOW_ID}/publish`;

describe('PublishDialog', () => {
  // A publication is based on the entity tag of the draft, which the dialog reads when the editor has not cached it.
  beforeEach(() => {
    server.use(
      http.get(`/api/v1/flows/${EXAMPLE_FLOW_ID}/draft`, () =>
        HttpResponse.json(
          { flowId: EXAMPLE_FLOW_ID, revision: 3, document: { format: 'orcon/flow@1', id: 'flow', name: 'Order notifications', settings: {}, variables: [], nodes: [], edges: [], editor: {} }, updatedAt: '2026-10-13T16:00:00Z' },
          { headers: { ETag: '"3"' } },
        ),
      ),
    );
  });

  it('publishes the draft with its release note and closes', async () => {
    let body: unknown = null;
    server.use(
      http.post(publishPath, async ({ request }) => {
        body = await request.json();
        return HttpResponse.json(
          { flowId: EXAMPLE_FLOW_ID, version: 3, contentHash: 'sha256:abc', publishedAt: '2026-10-14T10:00:00Z', note: 'Fixes refunds' },
          { status: 200 },
        );
      }),
    );
    const onOpenChange = vi.fn();
    renderInApp(<PublishDialog open onOpenChange={onOpenChange} flowId={EXAMPLE_FLOW_ID} />);
    await userEvent.type(screen.getByLabelText(/^Release note/), '  Fixes refunds  ');
    await userEvent.click(screen.getByRole('button', { name: 'Publish' }));
    await waitFor(() => {
      expect(onOpenChange).toHaveBeenCalledWith(false);
    });
    expect(body).toEqual({ note: 'Fixes refunds' });
  });

  it('publishes without a note when the note is left empty', async () => {
    let body: unknown = null;
    server.use(
      http.post(publishPath, async ({ request }) => {
        body = await request.json();
        return HttpResponse.json({ flowId: EXAMPLE_FLOW_ID, version: 4, contentHash: 'sha256:def', publishedAt: '2026-10-14T10:00:00Z' }, { status: 200 });
      }),
    );
    const onOpenChange = vi.fn();
    renderInApp(<PublishDialog open onOpenChange={onOpenChange} flowId={EXAMPLE_FLOW_ID} />);
    await userEvent.click(screen.getByRole('button', { name: 'Publish' }));
    await waitFor(() => {
      expect(onOpenChange).toHaveBeenCalledWith(false);
    });
    expect(body).toEqual({});
  });

  it('shows the problem of a draft the server refuses and stays open', async () => {
    server.use(
      http.post(publishPath, () =>
        HttpResponse.json(
          { type: 'about:blank', status: 422, title: 'The draft has errors', code: 'validation-failed', detail: 'Node Slack has no channel.' },
          { status: 422, headers: { 'Content-Type': 'application/problem+json' } },
        ),
      ),
    );
    const onOpenChange = vi.fn();
    renderInApp(<PublishDialog open onOpenChange={onOpenChange} flowId={EXAMPLE_FLOW_ID} />);
    await userEvent.click(screen.getByRole('button', { name: 'Publish' }));
    expect(await screen.findByRole('alert')).toBeVisible();
    expect(onOpenChange).not.toHaveBeenCalledWith(false);
  });
});
