// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { afterEach, describe, expect, it, vi } from 'vitest';
import { z } from 'zod';

import { readClipboardJson, writeClipboardJson, writeClipboardText } from './clipboard';

function stubClipboard(clipboard: { readText?: () => Promise<string>; writeText?: (text: string) => Promise<void> }) {
  vi.stubGlobal('navigator', { clipboard });
}

afterEach(() => {
  vi.unstubAllGlobals();
});

const fragment = z.object({ format: z.literal('orcon/flow-fragment@1'), nodes: z.array(z.string()) });

describe('writeClipboardText', () => {
  it('writes the text and reports success', async () => {
    const writeText = vi.fn(() => Promise.resolve());
    stubClipboard({ writeText });
    expect(await writeClipboardText('hello')).toBe(true);
    expect(writeText).toHaveBeenCalledWith('hello');
  });

  it('reports failure when the browser refuses', async () => {
    stubClipboard({ writeText: () => Promise.reject(new DOMException('Denied', 'NotAllowedError')) });
    expect(await writeClipboardText('hello')).toBe(false);
  });
});

describe('writeClipboardJson', () => {
  it('writes the value as JSON', async () => {
    const writeText = vi.fn(() => Promise.resolve());
    stubClipboard({ writeText });
    expect(await writeClipboardJson({ a: 1 })).toBe(true);
    expect(writeText).toHaveBeenCalledWith('{"a":1}');
  });
});

describe('readClipboardJson', () => {
  const read = (text: string) => {
    stubClipboard({ readText: () => Promise.resolve(text) });
    return readClipboardJson(fragment);
  };

  it('returns a document of the expected shape', async () => {
    expect(await read('{"format":"orcon/flow-fragment@1","nodes":["a"]}')).toEqual({
      ok: true,
      value: { format: 'orcon/flow-fragment@1', nodes: ['a'] },
    });
  });

  it('says why nothing was read', async () => {
    expect(await read('   ')).toEqual({ ok: false, reason: 'empty' });
    expect(await read('plain text')).toEqual({ ok: false, reason: 'not-json' });
    expect(await read('{"format":"other","nodes":[]}')).toEqual({ ok: false, reason: 'invalid' });
  });

  it('reports a refused read as denied', async () => {
    stubClipboard({ readText: () => Promise.reject(new DOMException('Denied', 'NotAllowedError')) });
    expect(await readClipboardJson(fragment)).toEqual({ ok: false, reason: 'denied' });
  });
});
