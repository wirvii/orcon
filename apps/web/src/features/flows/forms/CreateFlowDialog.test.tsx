// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { HttpResponse } from 'msw';
import { describe, expect, it } from 'vitest';

import { flow, productApi, project } from '../../../../test/product-fixtures';
import { renderRoute, server } from '../../../../test/screen-harness';
import { acme, signedIn } from '../../../../test/session-fixtures';

const newFlowId = '0195f1a0-0000-7000-8000-0000000000e2';

describe('CreateFlowDialog', () => {
  it('creates the flow and opens the editor', async () => {
    server.use(
      signedIn.currentUser,
      signedIn.organizations([acme]),
      productApi.projects([project]),
      productApi.flows([]),
      productApi.createFlow(() => HttpResponse.json({ ...flow, id: newFlowId, name: 'Refunds' }, { status: 201 })),
    );
    const { router } = renderRoute('/orgs/acme/projects/ventas/flows');
    await screen.findByRole('heading', { name: 'No flows yet' });
    await userEvent.click(screen.getAllByRole('button', { name: 'New flow' })[0] as HTMLElement);
    await screen.findByRole('dialog', { name: 'New flow' });
    await userEvent.type(screen.getByLabelText(/^Name/), 'Refunds');
    await userEvent.click(screen.getByRole('button', { name: 'Create and open' }));

    await waitFor(() => {
      expect(router.state.location.pathname).toBe(`/flows/${newFlowId}`);
    });
  });
});
