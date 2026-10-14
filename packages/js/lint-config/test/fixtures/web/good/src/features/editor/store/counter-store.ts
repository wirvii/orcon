// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { next, type Counter } from '../model/counter';

const initial: Counter = { value: 0 };

/** The editor store uses the editor model; its screens and components read it (orcon-web ST-6). */
export const counterStore = {
  state: initial,
  increment() {
    this.state = next(this.state);
  },
};
