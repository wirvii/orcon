// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { afterEach, describe, expect, it, vi } from 'vitest';
import { z } from 'zod';

import { clipboardEventText, parsePastedJson, readClipboardJson, setClipboardEventText, writeClipboardJson, writeClipboardText } from './clipboard';

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

describe('the clipboard of a copy, cut or paste event', () => {
  const board = () => {
    const data = new Map<string, string>();
    return { setData: (type: string, value: string) => data.set(type, value), getData: (type: string) => data.get(type) ?? '', data };
  };

  it('reads the text of the event and writes text into it', () => {
    const clipboardData = board();
    expect(setClipboardEventText({ clipboardData }, 'copied')).toBe(true);
    expect(clipboardEventText({ clipboardData })).toBe('copied');
  });

  it('copes with an event that carries no clipboard', () => {
    expect(clipboardEventText({ clipboardData: null })).toBe('');
    expect(setClipboardEventText({ clipboardData: null }, 'copied')).toBe(false);
  });
});

describe('parsePastedJson', () => {
  it('parses JSON of any shape and leaves its meaning to the caller', () => {
    expect(parsePastedJson('{"a":[1]}', 100)).toEqual({ ok: true, value: { a: [1] } });
    expect(parsePastedJson('3', 100)).toEqual({ ok: true, value: 3 });
  });

  it('says why there is nothing to use: empty, too large (counted in bytes, before parsing) or not JSON', () => {
    expect(parsePastedJson('  \n', 100)).toEqual({ ok: false, reason: 'empty' });
    expect(parsePastedJson('"áéíóú"', 8)).toEqual({ ok: false, reason: 'too-large' });
    expect(parsePastedJson('"áéíóú"', 12)).toMatchObject({ ok: true });
    expect(parsePastedJson('not json', 100)).toEqual({ ok: false, reason: 'not-json' });
  });
});
