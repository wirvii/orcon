// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { queryOptions } from '@tanstack/react-query';

import { sessionKeyRoot } from '../../../app/session/current-user';
import { getUserWithTag, listSessions } from './operations';

/** Keys of the account data. The person is read before any organization is active, so they start with `session` (orcon-web DT-4). */
export const accountKeys = {
  sessions: [...sessionKeyRoot, 'sessions'] as const,
  profile: [...sessionKeyRoot, 'profile'] as const,
};

/** The live sessions of the person. Read afresh each time the screen opens: another browser may have signed in or out. */
export function sessionsQueryOptions() {
  return queryOptions({ queryKey: accountKeys.sessions, queryFn: listSessions, staleTime: 0 });
}

/** The person with the entity tag a rename must send. Always read afresh: the tag is only good while nobody else changed the account. */
export function profileQueryOptions() {
  return queryOptions({ queryKey: accountKeys.profile, queryFn: getUserWithTag, staleTime: 0, gcTime: 0 });
}
