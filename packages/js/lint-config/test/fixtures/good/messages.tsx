// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { FormattedMessage, useIntl } from 'react-intl';

export const View = () => {
  const intl = useIntl();
  return (
    <section>
      <h1>
        <FormattedMessage id="flows.title" description="Page heading" defaultMessage="Flows" />
      </h1>
      <button
        type="button"
        aria-label={intl.formatMessage({
          id: 'flows.close',
          description: 'Close button label',
          defaultMessage: 'Close',
        })}
      />
      <p>
        <FormattedMessage
          id="flows.count"
          description="Number of flows"
          defaultMessage="{count, plural, one {# flow} other {# flows}}"
          values={{ count: 2 }}
        />
      </p>
    </section>
  );
};
