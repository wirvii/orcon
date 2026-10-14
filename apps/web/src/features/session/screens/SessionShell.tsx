// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { ReactNode } from 'react';

import { PageShell } from '../../../app/shell/PageShell';
import { UserMenu } from './UserMenu';

/** The frame of the signed-in pages: the shared navigation, plus the menu of the person who is signed in. */
export function SessionShell({ children }: { children: ReactNode }) {
  return <PageShell actions={<UserMenu />}>{children}</PageShell>;
}
