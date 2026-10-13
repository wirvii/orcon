// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

import { describe, expect, it } from 'vitest';

import { ownedRows, parityRows } from '../../src/parity/rows';

// The parity table lives in the skill. The list in code must say the same, row for row.
const markdown = readFileSync(resolve(__dirname, '../../../../../.agents/skills/orcon-ui/references/canvas.md'), 'utf8');
const section = markdown.slice(markdown.indexOf('## Parity'));
const table = [...section.matchAll(/^\| (P-\d{2}) \| ([^|]*)\| ([^|]*)\| ([^|]*)\| ([^|]*)\|$/gm)].map((match) => ({
  id: match[1],
  // "1 per move", "0 (visible)", "as each row": the number that starts the cell.
  max: /^\d+/.exec(match[4]!.trim())?.[0] ?? '1',
  owner: match[5]!.trim(),
}));

describe('parity rows', () => {
  it('has the 30 rows of the table', () => {
    expect(table).toHaveLength(30);
    expect(parityRows).toHaveLength(30);
  });

  it('matches the table: ids, owners and maximum steps', () => {
    for (const row of table) {
      const mine = parityRows.find((candidate) => candidate.id === row.id);
      expect(mine, row.id).toBeDefined();
      expect(mine?.owner, `${row.id} owner`).toBe(row.owner);
      expect(String(mine?.maxSteps), `${row.id} max steps`).toBe(row.max);
    }
  });

  it('owns the rows whose owner starts with SPEC-007', () => {
    expect(ownedRows.map((row) => row.id)).toEqual(table.filter((row) => row.owner.startsWith('SPEC-007')).map((row) => row.id));
  });
});
