// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * The keyboard map of the flow editor (SPEC-014 R-ED-KEY-01, docs/reference/editor-keyboard.md,
 * ADR-0082). It is the single source: the handlers resolve a key press through `resolveAction`,
 * and the `?` dialog lists `EDITOR_KEYMAP`. Each entry names the row of the reference document it
 * comes from, and a test keeps the two in step.
 */

export type KeyScope = 'editor' | 'canvas' | 'expression' | 'data-panel' | 'compare';

export type EditorAction =
  | 'move-focus'
  | 'navigate'
  | 'open-details'
  | 'dismiss'
  | 'nudge'
  | 'add-node'
  | 'connect'
  | 'delete'
  | 'rename'
  | 'toggle-disabled'
  | 'pin'
  | 'add-note'
  | 'tidy-up'
  | 'select-all'
  | 'clipboard'
  | 'duplicate'
  | 'undo'
  | 'redo'
  | 'save'
  | 'test-flow'
  | 'test-to-node'
  | 'test-node'
  | 'find'
  | 'zoom'
  | 'outline'
  | 'commands'
  | 'shortcuts'
  | 'autocomplete'
  | 'large-expression'
  | 'insert-field'
  | 'copy-field-path'
  | 'next-change';

/** `editor`: the editor handles the key itself. `canvas`: the design system canvas already handles it. `browser`: the browser does. */
export type KeyHandler = 'editor' | 'canvas' | 'browser';

export interface KeymapEntry {
  readonly action: EditorAction;
  /** The cell of the reference document's "Keys" column this entry comes from. */
  readonly row: string;
  /** Every chord that triggers the action; `Mod` is Ctrl, or Cmd on macOS. */
  readonly chords: readonly string[];
  /** The chords the dialog shows, when listing all of them would not help (the eight nudge chords). */
  readonly display?: readonly string[];
  readonly scope: KeyScope;
  readonly handler: KeyHandler;
  /** The slice that delivers the action; the `?` dialog lists only what this release has. */
  readonly slice: 'ED5a' | 'later';
}

export const EDITOR_KEYMAP: readonly KeymapEntry[] = [
  { action: 'move-focus', row: 'Tab / Shift+Tab', chords: ['Tab', 'Shift+Tab'], scope: 'editor', handler: 'browser', slice: 'ED5a' },
  { action: 'navigate', row: 'Arrow keys', chords: ['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight'], scope: 'canvas', handler: 'canvas', slice: 'ED5a' },
  { action: 'open-details', row: 'Enter', chords: ['Enter'], scope: 'canvas', handler: 'canvas', slice: 'ED5a' },
  { action: 'dismiss', row: 'Esc', chords: ['Escape'], scope: 'editor', handler: 'canvas', slice: 'ED5a' },
  { action: 'nudge', row: 'Shift+Arrow / Alt+Shift+Arrow', chords: ['Shift+ArrowUp', 'Shift+ArrowDown', 'Shift+ArrowLeft', 'Shift+ArrowRight', 'Alt+Shift+ArrowUp', 'Alt+Shift+ArrowDown', 'Alt+Shift+ArrowLeft', 'Alt+Shift+ArrowRight'], display: ['Shift+ArrowUp', 'Alt+Shift+ArrowUp'], scope: 'canvas', handler: 'canvas', slice: 'ED5a' },
  { action: 'add-node', row: 'A', chords: ['A'], scope: 'canvas', handler: 'canvas', slice: 'ED5a' },
  { action: 'connect', row: 'C', chords: ['C'], scope: 'canvas', handler: 'canvas', slice: 'ED5a' },
  { action: 'delete', row: 'Delete / Backspace', chords: ['Delete', 'Backspace'], scope: 'canvas', handler: 'canvas', slice: 'ED5a' },
  { action: 'rename', row: 'F2', chords: ['F2'], scope: 'canvas', handler: 'canvas', slice: 'ED5a' },
  { action: 'toggle-disabled', row: 'D', chords: ['D'], scope: 'canvas', handler: 'canvas', slice: 'ED5a' },
  { action: 'pin', row: 'P', chords: ['P'], scope: 'canvas', handler: 'editor', slice: 'later' },
  { action: 'add-note', row: 'Shift+S', chords: ['Shift+S'], scope: 'canvas', handler: 'canvas', slice: 'ED5a' },
  { action: 'tidy-up', row: 'Shift+Alt+T', chords: ['Shift+Alt+T'], scope: 'canvas', handler: 'editor', slice: 'ED5a' },
  { action: 'select-all', row: 'Ctrl+A', chords: ['Mod+A'], scope: 'canvas', handler: 'canvas', slice: 'ED5a' },
  { action: 'clipboard', row: 'Ctrl+C / Ctrl+X / Ctrl+V', chords: ['Mod+C', 'Mod+X', 'Mod+V'], scope: 'canvas', handler: 'editor', slice: 'ED5a' },
  { action: 'duplicate', row: 'Ctrl+D', chords: ['Mod+D'], scope: 'canvas', handler: 'editor', slice: 'ED5a' },
  { action: 'undo', row: 'Ctrl+Z / Ctrl+Shift+Z or Ctrl+Y', chords: ['Mod+Z'], scope: 'editor', handler: 'editor', slice: 'ED5a' },
  { action: 'redo', row: 'Ctrl+Z / Ctrl+Shift+Z or Ctrl+Y', chords: ['Mod+Shift+Z', 'Mod+Y'], scope: 'editor', handler: 'editor', slice: 'ED5a' },
  { action: 'save', row: 'Ctrl+S', chords: ['Mod+S'], scope: 'editor', handler: 'editor', slice: 'ED5a' },
  { action: 'test-flow', row: 'Ctrl+Enter', chords: ['Mod+Enter'], scope: 'editor', handler: 'editor', slice: 'later' },
  { action: 'test-to-node', row: 'Ctrl+Shift+Enter', chords: ['Mod+Shift+Enter'], scope: 'canvas', handler: 'editor', slice: 'later' },
  { action: 'test-node', row: 'R', chords: ['R'], scope: 'canvas', handler: 'editor', slice: 'later' },
  { action: 'find', row: 'Ctrl+F', chords: ['Mod+F'], scope: 'editor', handler: 'editor', slice: 'ED5a' },
  { action: 'zoom', row: '1 / 0 / + / -', chords: ['1', '0', '+', '-'], scope: 'canvas', handler: 'canvas', slice: 'ED5a' },
  { action: 'outline', row: 'O', chords: ['O'], scope: 'canvas', handler: 'editor', slice: 'ED5a' },
  { action: 'commands', row: 'Ctrl+K', chords: ['Mod+K'], scope: 'editor', handler: 'editor', slice: 'ED5a' },
  { action: 'shortcuts', row: '?', chords: ['?'], scope: 'canvas', handler: 'editor', slice: 'ED5a' },
  { action: 'autocomplete', row: 'Ctrl+Space', chords: ['Mod+Space'], scope: 'expression', handler: 'editor', slice: 'later' },
  { action: 'large-expression', row: 'Alt+Enter', chords: ['Alt+Enter'], scope: 'expression', handler: 'editor', slice: 'later' },
  { action: 'insert-field', row: 'Enter (on a field in the input panel)', chords: ['Enter'], scope: 'data-panel', handler: 'editor', slice: 'later' },
  { action: 'copy-field-path', row: 'Ctrl+Shift+C (on a field)', chords: ['Mod+Shift+C'], scope: 'data-panel', handler: 'editor', slice: 'later' },
  { action: 'next-change', row: 'N / Shift+N', chords: ['N', 'Shift+N'], scope: 'compare', handler: 'editor', slice: 'later' },
];

/** The slice of the keymap this release delivers, in map order. */
export const AVAILABLE_ENTRIES: readonly KeymapEntry[] = EDITOR_KEYMAP.filter((entry) => entry.slice === 'ED5a');

/** What a key press carries; a DOM `KeyboardEvent` satisfies it. */
export interface KeyEventLike {
  readonly key: string;
  /** The physical key (`KeyT`); with Alt held, a Mac reports another character as `key`, so letters are matched by it. */
  readonly code?: string | undefined;
  readonly ctrlKey: boolean;
  readonly metaKey: boolean;
  readonly shiftKey: boolean;
  readonly altKey: boolean;
}

export interface Platform {
  readonly mac: boolean;
}

const MODIFIERS = new Set(['Mod', 'Ctrl', 'Shift', 'Alt']);

/** A letter or digit or sign that is written with one key and no name: its shift state is part of the character. */
const isCharacter = (key: string): boolean => key.length === 1;

/** True when the key press is the chord. Letters compare without case; `?`, `+`, `-`, digits ignore Shift. */
export function matchesChord(event: KeyEventLike, chord: string, platform: Platform): boolean {
  const parts = chord.split('+');
  // The chord "+" and a chord that ends in '+' ("Mod++") have an empty last piece: the key is the plus sign.
  const key = chord === '+' || chord.endsWith('++') ? '+' : (parts[parts.length - 1] ?? '');
  const modifiers = new Set(parts.filter((part) => MODIFIERS.has(part)));
  const wantsMod = modifiers.has('Mod');
  const primary = platform.mac ? event.metaKey : event.ctrlKey;
  const foreign = platform.mac ? event.ctrlKey : event.metaKey;
  // Ctrl and Cmd are one modifier ("Mod") by platform; the other one pressed means a different chord.
  if (foreign || wantsMod !== primary) return false;
  if (modifiers.has('Alt') !== event.altKey) return false;
  const letterOrNamed = !isCharacter(key) || /[a-z]/i.test(key);
  if (letterOrNamed && modifiers.has('Shift') !== event.shiftKey) return false;
  if (modifiers.has('Alt') && /^[a-z]$/i.test(key) && event.code !== undefined) return event.code === `Key${key.toUpperCase()}`;
  const pressed = event.key === ' ' ? 'Space' : event.key;
  return isCharacter(key) ? pressed.toLowerCase() === key.toLowerCase() : pressed === key;
}

/**
 * The map entry a key press triggers in the given scopes, or undefined. With `handler` only the
 * entries handled that way are considered; entries of later slices are included only with
 * `includeLater`, so the editor can say they are not available yet instead of letting another
 * handler act on the key.
 */
export function resolveEntry(event: KeyEventLike, scopes: readonly KeyScope[], platform: Platform, options: { handler?: KeyHandler; includeLater?: boolean } = {}): KeymapEntry | undefined {
  for (const entry of EDITOR_KEYMAP) {
    if ((entry.slice !== 'ED5a' && !options.includeLater) || !scopes.includes(entry.scope)) continue;
    if (options.handler !== undefined && entry.handler !== options.handler) continue;
    if (entry.chords.some((chord) => matchesChord(event, chord, platform))) return entry;
  }
  return undefined;
}

/** A character shortcut (`A`, `Shift+S`, `?`): it works only while the canvas has focus and never in a text field (WCAG 2.1.4). */
export const isCharacterShortcut = (chord: string): boolean => !chord.includes('Mod') && (chord === '+' || chord.endsWith('++') || (chord.split('+').pop() ?? '').length === 1);

const ARROWS: Record<string, string> = { ArrowUp: '↑', ArrowDown: '↓', ArrowLeft: '←', ArrowRight: '→' };

/** The words of a chord for display: `Mod+Shift+Z` becomes ["Ctrl", "Shift", "Z"], or ["Cmd", ...] on macOS. */
export function chordKeys(chord: string, platform: Platform): string[] {
  if (chord === '+') return ['+'];
  const parts = chord.endsWith('++') ? [...chord.slice(0, -2).split('+'), '+'] : chord.split('+');
  return parts.map((part) => (part === 'Mod' ? (platform.mac ? 'Cmd' : 'Ctrl') : (ARROWS[part] ?? part)));
}
