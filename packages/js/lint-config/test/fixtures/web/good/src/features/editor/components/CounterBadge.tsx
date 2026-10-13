// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { counterStore } from '../store/counter-store';

export function CounterBadge() {
  return <output>{counterStore.state.value}</output>;
}
