// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { layoutGraph, type LayoutInput } from './layout';

// The worker's whole job: receive the plain graph `layout-client.ts` sends and answer with positions.
self.onmessage = (event: MessageEvent<LayoutInput>) => {
  self.postMessage(layoutGraph(event.data));
};
