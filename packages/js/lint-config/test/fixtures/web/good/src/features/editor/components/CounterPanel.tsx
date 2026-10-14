// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { CounterField } from '../forms/CounterField';

// A component of the editor may compose the form renderer (the node details view wires the node form).
export function CounterPanel() {
  return <CounterField />;
}
