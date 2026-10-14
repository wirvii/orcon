// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from 'vitest';

import { EXAMPLE_FLOW_ID, exampleFlowDocument } from '../model/example-flow';
import { cloneDocument } from '../model/flow-document';

import { createLocalDraftSource, DraftNotFound } from './local-draft-source';

describe('the local draft source', () => {
  it('opens the sample flow at revision 1 and hands out copies, so reading never changes what is held', async () => {
    const source = createLocalDraftSource();
    const first = await source.load(EXAMPLE_FLOW_ID);
    expect(first.revision).toBe('1');
    expect(first.document).toEqual(exampleFlowDocument());
    first.document.name = 'Changed in the reader';
    expect((await source.load(EXAMPLE_FLOW_ID)).document.name).toBe('Order notifications');
  });

  it('saves a draft that is still at the revision it started from and moves the revision on', async () => {
    const source = createLocalDraftSource();
    const draft = cloneDocument(exampleFlowDocument());
    draft.name = 'Renamed';
    expect(await source.save(EXAMPLE_FLOW_ID, draft, '1')).toEqual({ kind: 'saved', revision: '2' });
    expect(await source.load(EXAMPLE_FLOW_ID)).toMatchObject({ revision: '2', document: { name: 'Renamed' } });
  });

  it('never overwrites a draft that moved on: a stale revision is a conflict and changes nothing', async () => {
    const source = createLocalDraftSource();
    const draft = cloneDocument(exampleFlowDocument());
    await source.save(EXAMPLE_FLOW_ID, draft, '1');
    draft.name = 'Late edit';
    expect(await source.save(EXAMPLE_FLOW_ID, draft, '1')).toEqual({ kind: 'conflict' });
    expect((await source.load(EXAMPLE_FLOW_ID)).document.name).toBe('Order notifications');
  });

  it('answers a flow that does not exist as the server would: a problem with status 404', async () => {
    const source = createLocalDraftSource();
    await expect(source.load('01930000-0000-7000-8000-000000000000')).rejects.toBeInstanceOf(DraftNotFound);
    await expect(source.save('01930000-0000-7000-8000-000000000000', exampleFlowDocument(), '1')).rejects.toMatchObject({ status: 404, code: 'not-found' });
  });

  it('builds a draft from an asynchronous seed once', async () => {
    let calls = 0;
    const source = createLocalDraftSource(
      new Map([
        [
          'async-flow',
          async () => {
            calls += 1;
            return Promise.resolve(exampleFlowDocument());
          },
        ],
      ]),
    );
    await source.load('async-flow');
    await source.load('async-flow');
    expect(calls).toBe(1);
  });
});
