// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { ApiProblem } from '../../../app/api/http';

import type { DraftSource, SaveResult } from './draft-source';
import { getDraft, replaceDraft } from './operations';

/**
 * The draft source of the running application: drafts live on the server (SPEC-014 ED5a part 2).
 * A save sends the revision it started from as `If-Match`; when the server holds a newer draft it
 * answers 412, which is reported as a conflict so the editor never overwrites the other change.
 * Every other failure is thrown and the autosave retries it later.
 */
export function createServerDraftSource(): DraftSource {
  return {
    load: async (flowId) => {
      const draft = await getDraft(flowId);
      return { document: draft.document, revision: draft.etag };
    },
    save: async (flowId, document, ifMatch): Promise<SaveResult> => {
      try {
        const revision = await replaceDraft(flowId, document, ifMatch);
        return { kind: 'saved', revision };
      } catch (error) {
        if (error instanceof ApiProblem && error.status === 412) return { kind: 'conflict' };
        throw error;
      }
    },
  };
}
