// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { useState } from 'react';

import { sortFlows, type Flow } from '../model/flow';

export function FlowPicker({ flows, label, onPick }: { flows: readonly Flow[]; label: string; onPick: (id: string) => void }) {
  const [selected, setSelected] = useState('');
  return (
    <label>
      {label}
      <select
        value={selected}
        onChange={(event) => {
          setSelected(event.target.value);
          onPick(event.target.value);
        }}
      >
        {sortFlows(flows).map((flow) => (
          <option key={flow.id} value={flow.id}>
            {flow.name}
          </option>
        ))}
      </select>
    </label>
  );
}
