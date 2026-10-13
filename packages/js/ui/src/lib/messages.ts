// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { defineMessages } from 'react-intl';

/** Messages shared by several components. */
export const commonMessages = defineMessages({
  close: { id: 'common.close', defaultMessage: 'Close', description: 'Accessible name of a button that closes a dialog, sheet or panel.' },
  cancel: { id: 'common.cancel', defaultMessage: 'Cancel', description: 'Button that dismisses a dialog without doing anything.' },
});
