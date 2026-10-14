// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

import { describe, expect, it } from 'vitest';

import { AVAILABLE_ENTRIES, EDITOR_KEYMAP, chordKeys, isCharacterShortcut, matchesChord, resolveEntry, type KeyEventLike } from './keymap';

const here = dirname(fileURLToPath(import.meta.url));
const referencePath = resolve(here, '../../../../../../docs/reference/editor-keyboard.md');

const press = (key: string, modifiers: Partial<Omit<KeyEventLike, 'key'>> = {}): KeyEventLike => ({
  key,
  ctrlKey: false,
  metaKey: false,
  shiftKey: false,
  altKey: false,
  ...modifiers,
});
const pc = { mac: false };
const mac = { mac: true };

describe('the keymap and its reference document', () => {
  it('has one entry for each row of docs/reference/editor-keyboard.md, with the same keys', () => {
    const rows = readFileSync(referencePath, 'utf8')
      .split('\n')
      .filter((line) => line.startsWith('| ') && !line.startsWith('| Keys') && !line.startsWith('|---'))
      .map((line) => (line.split('|')[1] ?? '').trim());
    const mapped = [...new Set(EDITOR_KEYMAP.map((entry) => entry.row))];
    expect(mapped.sort()).toEqual([...new Set(rows)].sort());
  });

  it('keeps every action once per row and lists only what this release delivers in the dialog', () => {
    expect(new Set(EDITOR_KEYMAP.map((entry) => entry.action)).size).toBe(EDITOR_KEYMAP.length);
    expect(AVAILABLE_ENTRIES.every((entry) => entry.slice === 'ED5a')).toBe(true);
    expect(AVAILABLE_ENTRIES.map((entry) => entry.action)).not.toContain('pin');
    expect(AVAILABLE_ENTRIES.map((entry) => entry.action)).toContain('tidy-up');
  });
});

describe('matching a key press to a chord', () => {
  it('reads Mod as Ctrl, or as Cmd on a Mac, and nothing else', () => {
    expect(matchesChord(press('z', { ctrlKey: true }), 'Mod+Z', pc)).toBe(true);
    expect(matchesChord(press('z', { metaKey: true }), 'Mod+Z', pc)).toBe(false);
    expect(matchesChord(press('z', { metaKey: true }), 'Mod+Z', mac)).toBe(true);
    expect(matchesChord(press('z', { ctrlKey: true }), 'Mod+Z', mac)).toBe(false);
    expect(matchesChord(press('z'), 'Mod+Z', pc)).toBe(false);
  });

  it('tells Ctrl+Z from Ctrl+Shift+Z and a letter from the same letter with a modifier', () => {
    expect(matchesChord(press('Z', { ctrlKey: true, shiftKey: true }), 'Mod+Shift+Z', pc)).toBe(true);
    expect(matchesChord(press('Z', { ctrlKey: true, shiftKey: true }), 'Mod+Z', pc)).toBe(false);
    expect(matchesChord(press('a'), 'A', pc)).toBe(true);
    expect(matchesChord(press('a', { ctrlKey: true }), 'A', pc)).toBe(false);
    expect(matchesChord(press('a', { altKey: true }), 'A', pc)).toBe(false);
    expect(matchesChord(press('S', { shiftKey: true }), 'Shift+S', pc)).toBe(true);
    expect(matchesChord(press('s'), 'Shift+S', pc)).toBe(false);
  });

  it('ignores Shift for characters that need it to be typed', () => {
    expect(matchesChord(press('?', { shiftKey: true }), '?', pc)).toBe(true);
    expect(matchesChord(press('?'), '?', pc)).toBe(true);
    expect(matchesChord(press('+', { shiftKey: true }), '+', pc)).toBe(true);
    expect(matchesChord(press('-'), '-', pc)).toBe(true);
    expect(matchesChord(press('1'), '1', pc)).toBe(true);
  });

  it('matches a letter pressed with Alt by the physical key, because a Mac types another character then', () => {
    expect(matchesChord(press('ˇ', { altKey: true, shiftKey: true, code: 'KeyT' }), 'Shift+Alt+T', mac)).toBe(true);
    expect(matchesChord(press('t', { altKey: true, shiftKey: true, code: 'KeyY' }), 'Shift+Alt+T', mac)).toBe(false);
    expect(matchesChord(press('t', { altKey: true, shiftKey: true }), 'Shift+Alt+T', pc)).toBe(true);
  });

  it('matches named keys, the space bar and the chord of a plus sign', () => {
    expect(matchesChord(press('ArrowUp', { shiftKey: true }), 'Shift+ArrowUp', pc)).toBe(true);
    expect(matchesChord(press('ArrowUp'), 'Shift+ArrowUp', pc)).toBe(false);
    expect(matchesChord(press('ArrowLeft', { altKey: true, shiftKey: true }), 'Alt+Shift+ArrowLeft', pc)).toBe(true);
    expect(matchesChord(press(' ', { ctrlKey: true }), 'Mod+Space', pc)).toBe(true);
    expect(matchesChord(press('Enter', { ctrlKey: true }), 'Mod+Enter', pc)).toBe(true);
    expect(matchesChord(press('Enter', { ctrlKey: true, shiftKey: true }), 'Mod+Enter', pc)).toBe(false);
    expect(matchesChord(press('+', { ctrlKey: true }), 'Mod++', pc)).toBe(true);
    expect(matchesChord(press('F2'), 'F2', pc)).toBe(true);
  });
});

describe('resolving a key press', () => {
  it('finds the entry for the scopes in play, and only entries of this release unless asked', () => {
    expect(resolveEntry(press('z', { ctrlKey: true }), ['editor', 'canvas'], pc)?.action).toBe('undo');
    expect(resolveEntry(press('y', { ctrlKey: true }), ['editor', 'canvas'], pc)?.action).toBe('redo');
    expect(resolveEntry(press('Z', { ctrlKey: true, shiftKey: true }), ['editor'], pc)?.action).toBe('redo');
    expect(resolveEntry(press('t', { shiftKey: true, altKey: true }), ['canvas'], pc, { handler: 'editor' })?.action).toBe('tidy-up');
    expect(resolveEntry(press('p'), ['canvas'], pc)).toBeUndefined();
    expect(resolveEntry(press('p'), ['canvas'], pc, { includeLater: true })).toMatchObject({ action: 'pin', slice: 'later' });
    expect(resolveEntry(press('a'), ['canvas'], pc, { handler: 'editor' })).toBeUndefined();
    expect(resolveEntry(press('a'), ['canvas'], pc, { handler: 'canvas' })?.action).toBe('add-node');
    expect(resolveEntry(press('q'), ['canvas', 'editor'], pc)).toBeUndefined();
    expect(resolveEntry(press('z', { ctrlKey: true }), ['expression'], pc)).toBeUndefined();
  });
});

describe('describing chords', () => {
  it('tells single-character shortcuts from the others', () => {
    expect(['A', 'Shift+S', 'Shift+Alt+T', '?', '+', '1'].map(isCharacterShortcut)).toEqual([true, true, true, true, true, true]);
    expect(['Mod+Z', 'F2', 'Delete', 'Shift+ArrowUp', 'Escape', 'Mod++'].map(isCharacterShortcut)).toEqual([false, false, false, false, false, false]);
  });

  it('writes the keys for the platform', () => {
    expect(chordKeys('Mod+Shift+Z', pc)).toEqual(['Ctrl', 'Shift', 'Z']);
    expect(chordKeys('Mod+Shift+Z', mac)).toEqual(['Cmd', 'Shift', 'Z']);
    expect(chordKeys('Shift+ArrowUp', pc)).toEqual(['Shift', '↑']);
    expect(chordKeys('Mod++', pc)).toEqual(['Ctrl', '+']);
    expect(chordKeys('+', pc)).toEqual(['+']);
  });
});
