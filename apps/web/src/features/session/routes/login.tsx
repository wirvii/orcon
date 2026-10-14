// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createFileRoute } from '@tanstack/react-router';

import { RouteError } from '../../../app/errors/RouteError';
import { loginSearchSchema } from '../model/params';
import { SignInScreen } from '../screens/SignInScreen';

// The sign-in page is the one page a person without a session can open. It reads no server data:
// the session is checked by the layout of the signed-in pages, not here.
export const Route = createFileRoute('/login')({
  validateSearch: loginSearchSchema,
  errorComponent: RouteError,
  component: SignInScreen,
});
