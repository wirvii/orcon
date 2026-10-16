// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { ComingSoon } from '@orcon/ui';
import { Code } from 'lucide-react';
import { useEffect } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { markFirstUsefulScreen } from '../../../app/perf/marks';

const messages = defineMessages({
  title: { id: 'settings.apiKeys.title', defaultMessage: 'API keys', description: 'Title of the card that says API keys are not available yet.' },
  text: {
    id: 'settings.apiKeys.text',
    defaultMessage: 'Creating API keys to use Orcon from your own systems, the command line or an assistant is not available yet.',
    description: 'One sentence that says what is missing in the API keys section.',
  },
});

/** API keys: the server cannot create them yet, so the section says so and offers no button (SPEC-019 R-UX-SET-06). */
export function ApiKeysScreen() {
  const intl = useIntl();
  useEffect(() => {
    markFirstUsefulScreen('/orgs/$orgSlug/settings/api-keys');
  }, []);
  return <ComingSoon icon={Code} title={intl.formatMessage(messages.title)} description={intl.formatMessage(messages.text)} />;
}
