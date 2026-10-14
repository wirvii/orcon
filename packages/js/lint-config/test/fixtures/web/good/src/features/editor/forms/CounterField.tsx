// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { counterStore } from '../store/counter-store';

export function CounterField() {
  return <input readOnly value={counterStore.state.value} />;
}
