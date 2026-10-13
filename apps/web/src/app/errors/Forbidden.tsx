// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { EmptyState } from '@orcon/ui';
import { defineMessages, useIntl } from 'react-intl';

const messages = defineMessages({
  title: {
    id: 'app.error.forbidden.title',
    defaultMessage: 'You do not have permission to see this',
    description: 'Heading shown when the server refuses a request because of the permissions of the person.',
  },
  text: {
    id: 'app.error.forbidden.text',
    defaultMessage: 'Ask an administrator of your organization for access.',
    description: 'Explains what to do after a permission error: there is nothing to retry.',
  },
});

/** What a screen shows when the server answers 403: the reason and no retry (orcon-web ER-3). */
export function Forbidden() {
  const intl = useIntl();
  return <EmptyState title={intl.formatMessage(messages.title)} description={intl.formatMessage(messages.text)} />;
}
