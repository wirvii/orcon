// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { FormattedMessage } from 'react-intl';

export const View = () => (
  <FormattedMessage
    id="flows.count"
    description="Number of flows"
    defaultMessage="{count, plural, one {# flow}"
    values={{ count: 2 }}
  />
);
