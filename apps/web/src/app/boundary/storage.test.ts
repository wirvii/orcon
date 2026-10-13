// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { z } from 'zod';

import { readStored, removeStored, writeStored } from './storage';

/** An in-memory stand-in for localStorage that can be told to fail like a blocked browser. */
function fakeStorage() {
  const data = new Map<string, string>();
  let blocked = false;
  const guard = () => {
    if (blocked) throw new DOMException('Storage is blocked', 'SecurityError');
  };
  return {
    block: () => {
      blocked = true;
    },
    data,
    storage: {
      getItem: (key: string) => {
        guard();
        return data.get(key) ?? null;
      },
      setItem: (key: string, value: string) => {
        guard();
        data.set(key, value);
      },
      removeItem: (key: string) => {
        guard();
        data.delete(key);
      },
    },
  };
}

const schema = z.object({ pinned: z.array(z.string()) });
const fallback = { pinned: [] as string[] };

let fake: ReturnType<typeof fakeStorage>;

beforeEach(() => {
  fake = fakeStorage();
  vi.stubGlobal('window', { localStorage: fake.storage });
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('readStored', () => {
  it('returns the fallback when nothing is saved', () => {
    expect(readStored('k', schema, fallback)).toBe(fallback);
  });

  it('reads a saved value that matches the schema', () => {
    writeStored('k', { pinned: ['a', 'b'] });
    expect(readStored('k', schema, fallback)).toEqual({ pinned: ['a', 'b'] });
  });

  it('returns the fallback for broken JSON', () => {
    fake.data.set('k', '{not json');
    expect(readStored('k', schema, fallback)).toBe(fallback);
  });

  it('returns the fallback for a value that no longer matches the schema', () => {
    fake.data.set('k', JSON.stringify({ pinned: 'a' }));
    expect(readStored('k', schema, fallback)).toBe(fallback);
  });

  it('returns the fallback when the browser blocks storage', () => {
    fake.block();
    expect(readStored('k', schema, fallback)).toBe(fallback);
  });
});

describe('writeStored', () => {
  it('saves JSON and reports success', () => {
    expect(writeStored('k', { pinned: ['a'] })).toBe(true);
    expect(fake.data.get('k')).toBe('{"pinned":["a"]}');
  });

  it('reports failure when the browser blocks storage', () => {
    fake.block();
    expect(writeStored('k', { pinned: [] })).toBe(false);
  });
});

describe('removeStored', () => {
  it('removes a saved value', () => {
    writeStored('k', 1);
    removeStored('k');
    expect(fake.data.has('k')).toBe(false);
  });

  it('never throws when the browser blocks storage', () => {
    fake.block();
    expect(() => {
      removeStored('k');
    }).not.toThrow();
  });
});
