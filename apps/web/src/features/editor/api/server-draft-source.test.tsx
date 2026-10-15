// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { HttpResponse, http } from 'msw';
import { describe, expect, it } from 'vitest';

import { problem } from '../../../../test/session-fixtures';
import { server } from '../../../../test/screen-harness';
import { EXAMPLE_FLOW_ID, exampleFlowDocument } from '../model/example-flow';

import { createServerDraftSource } from './server-draft-source';

const draftPath = `/api/v1/flows/${EXAMPLE_FLOW_ID}/draft`;

/** A `FlowDraft` answer at a revision, with its entity tag. */
function draftAnswer(revision: number) {
  return HttpResponse.json(
    { flowId: EXAMPLE_FLOW_ID, revision, document: exampleFlowDocument(), updatedAt: '2026-10-14T10:00:00Z' },
    { headers: { ETag: `"${revision}"` } },
  );
}

describe('createServerDraftSource', () => {
  it('loads the draft and keeps its entity tag as the revision', async () => {
    server.use(http.get(draftPath, () => draftAnswer(3)));
    const loaded = await createServerDraftSource().load(EXAMPLE_FLOW_ID);
    expect(loaded.revision).toBe('"3"');
    expect(loaded.document.name).toBe(exampleFlowDocument().name);
  });

  it('saves only against the revision it started from, and returns the new one', async () => {
    let received: string | null = null;
    server.use(
      http.put(draftPath, ({ request }) => {
        received = request.headers.get('If-Match');
        return draftAnswer(4);
      }),
    );
    const result = await createServerDraftSource().save(EXAMPLE_FLOW_ID, exampleFlowDocument(), '"3"');
    expect(received).toBe('"3"');
    expect(result).toEqual({ kind: 'saved', revision: '"4"' });
  });

  it('reports a save over a newer draft as a conflict, without overwriting', async () => {
    server.use(http.put(draftPath, () => problem(412, 'precondition-failed', 'precondition-failed')));
    const result = await createServerDraftSource().save(EXAMPLE_FLOW_ID, exampleFlowDocument(), '"3"');
    expect(result).toEqual({ kind: 'conflict' });
  });

  it('lets any other failure reach the autosave, which retries it later', async () => {
    server.use(http.put(draftPath, () => problem(500, 'internal', 'internal')));
    await expect(createServerDraftSource().save(EXAMPLE_FLOW_ID, exampleFlowDocument(), '"3"')).rejects.toMatchObject({ status: 500 });
  });
});
