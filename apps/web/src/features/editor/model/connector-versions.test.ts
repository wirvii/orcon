// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from 'vitest';

import { availableUpdate, compareVersions, INSTALLED_CONNECTORS, pinOf } from './connector-versions';
import { exampleFlowDocument } from './example-flow';
import { findNode } from './flow-document';

const sample = (id: string) => {
  const node = findNode(exampleFlowDocument(), id);
  if (!node) throw new Error(`no node ${id}`);
  return node;
};

describe('connector versions', () => {
  it('compares versions number by number, not as text', () => {
    expect(compareVersions('1.10.0', '1.9.0')).toBeGreaterThan(0);
    expect(compareVersions('1.0.0', '1.0.0')).toBe(0);
    expect(compareVersions('0.9.9', '1.0.0')).toBeLessThan(0);
    expect(compareVersions('2', '1.9.9')).toBeGreaterThan(0);
  });

  it('reads the pin of a node', () => {
    expect(pinOf(sample('slack'))).toEqual({ version: '1.0.0', digest: INSTALLED_CONNECTORS['orcon.http@1']?.[0]?.digest });
    expect(pinOf(sample('webhook'))).toBeUndefined();
  });

  it('offers the newest installed version with every change since the pin', () => {
    const update = availableUpdate(sample('slack'));
    expect(update?.from).toBe('1.0.0');
    expect(update?.latest.version).toBe('1.2.0');
    expect(update?.releases.map((release) => release.version)).toEqual(['1.1.0', '1.2.0']);
    // A change that can alter behaviour asks for a check.
    expect(update?.needsReview).toBe(true);
  });

  it('offers nothing to a node that already uses the newest version, has no pin, or has an unknown type', () => {
    expect(availableUpdate(sample('inventory'))).toBeUndefined();
    expect(availableUpdate(sample('validate'))).toBeUndefined();
    const unknown = { ...sample('slack'), type: 'acme.unknown@1' };
    expect(availableUpdate(unknown)).toBeUndefined();
  });
});
