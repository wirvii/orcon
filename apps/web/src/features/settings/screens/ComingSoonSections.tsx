// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { ComingSoon } from '@orcon/ui';
import { Code, Users } from 'lucide-react';
import { useEffect } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { markFirstUsefulScreen } from '../../../app/perf/marks';

const messages = defineMessages({
  membersTitle: { id: 'settings.members.title', defaultMessage: 'People and roles', description: 'Title of the card that says the list of people of the organization is not available yet.' },
  membersText: {
    id: 'settings.members.text',
    defaultMessage: 'Seeing the people of the organization, inviting them and changing their roles are not available yet.',
    description: 'One sentence that says what is missing in the people and roles section.',
  },
  keysTitle: { id: 'settings.apiKeys.title', defaultMessage: 'API keys', description: 'Title of the card that says API keys are not available yet.' },
  keysText: {
    id: 'settings.apiKeys.text',
    defaultMessage: 'Creating API keys to use Orcon from your own systems is not available yet.',
    description: 'One sentence that says what is missing in the API keys section.',
  },
});

/** People and roles: the server has no list of people yet (SPEC-019 R-UX-SET-05). */
export function MembersScreen() {
  const intl = useIntl();
  useEffect(() => {
    markFirstUsefulScreen('/orgs/$orgSlug/settings/members');
  }, []);
  return <ComingSoon icon={Users} title={intl.formatMessage(messages.membersTitle)} description={intl.formatMessage(messages.membersText)} />;
}

/** API keys: the server cannot create them yet (SPEC-019 R-UX-SET-06). */
export function ApiKeysScreen() {
  const intl = useIntl();
  useEffect(() => {
    markFirstUsefulScreen('/orgs/$orgSlug/settings/api-keys');
  }, []);
  return <ComingSoon icon={Code} title={intl.formatMessage(messages.keysTitle)} description={intl.formatMessage(messages.keysText)} />;
}
