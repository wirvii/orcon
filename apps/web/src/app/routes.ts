// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { index, physical, rootRoute } from '@tanstack/virtual-file-routes';

/**
 * The route tree is built from virtual file routes: the root layout and the home
 * page live in `app/shell/`, and every feature mounts the `routes/` folder next to
 * its screens (orcon-web RT-1). A new feature adds one `physical` line here.
 */
export const routes = rootRoute('shell/RootLayout.tsx', [
  index('shell/Home.tsx'),
  physical('/settings', '../features/settings/routes'),
]);
