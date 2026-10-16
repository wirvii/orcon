// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Expression chips (SPEC-019 R-UX-NDV-03). A stored expression is a string that starts with `=` and holds text and
 * `{{ ... }}` blocks (ADR-0018 section 1). The chips are only a way to show it: a block whose content is a plain path
 * to a field (`$json.customer.phone`, `$('Webhook').item.json.id`) is shown as a readable chip, anything else as
 * raw text inside an "expression" chip. Nothing here changes the stored string: every segment keeps the exact
 * characters it was made from, so going from the text to the chips and back gives the same text, byte for byte,
 * for any input.
 */

export interface PathChip {
  readonly type: 'json' | 'node';
  /** Readable form: `customer.phone`, or `Webhook > id` for a field of another step. */
  readonly label: string;
  /** The steps into the data: names and array positions. */
  readonly path: readonly (string | number)[];
  /** The name of the step the field comes from, for `type: 'node'`. */
  readonly node?: string;
}

export interface RawChip {
  readonly type: 'raw';
  /** The expression as typed, without the braces and the spaces around it. */
  readonly label: string;
}

export type ExprChip = PathChip | RawChip;

export type Segment =
  | { readonly kind: 'text'; readonly raw: string }
  | { readonly kind: 'expr'; readonly raw: string; readonly chip: ExprChip };

export interface ParsedExpression {
  /** The stored text starts with `=`: it is an expression. Fixed text is one text segment. */
  readonly expression: boolean;
  readonly segments: readonly Segment[];
}

const IDENTIFIER = /^[A-Za-z_$][A-Za-z0-9_$]*/;

/** Reads a quoted string literal at `start` (the quote); returns its value and the index after it, or undefined when it is not closed. */
function readString(text: string, start: number): { value: string; end: number } | undefined {
  const quote = text.charAt(start);
  let value = '';
  let index = start + 1;
  while (index < text.length) {
    const char = text.charAt(index);
    if (char === '\\') {
      const next = text.charAt(index + 1);
      if (next === '') return undefined;
      value += next;
      index += 2;
    } else if (char === quote) {
      return { value, end: index + 1 };
    } else {
      value += char;
      index += 1;
    }
  }
  return undefined;
}

/** The accessors that follow a root: `.name`, `['name']`, `[0]`. Returns the whole path, or undefined when anything else appears. */
function readPath(text: string, from: number): (string | number)[] | undefined {
  const path: (string | number)[] = [];
  let index = from;
  while (index < text.length) {
    const char = text.charAt(index);
    if (char === '.') {
      const found = IDENTIFIER.exec(text.slice(index + 1));
      if (!found) return undefined;
      path.push(found[0]);
      index += 1 + found[0].length;
    } else if (char === '[') {
      const inner = text.charAt(index + 1);
      if (inner === "'" || inner === '"') {
        const read = readString(text, index + 1);
        if (!read || text.charAt(read.end) !== ']') return undefined;
        path.push(read.value);
        index = read.end + 1;
      } else {
        const found = /^\d+/.exec(text.slice(index + 1));
        if (!found || text.charAt(index + 1 + found[0].length) !== ']') return undefined;
        path.push(Number(found[0]));
        index += 2 + found[0].length;
      }
    } else {
      return undefined;
    }
  }
  return path;
}

/** `customer.phone`, `items[0].sku`: the readable form of a path. */
export function pathLabel(path: readonly (string | number)[]): string {
  return path.reduce<string>((label, step, index) => (typeof step === 'number' ? `${label}[${String(step)}]` : index === 0 ? step : `${label}.${step}`), '');
}

/** Classifies the inside of a `{{ ... }}` block. */
export function classify(inner: string): ExprChip {
  const text = inner.trim();
  const raw: RawChip = { type: 'raw', label: text };
  if (text === '') return raw;

  const direct = /^\$json|^\$input\.item\.json/.exec(text);
  if (direct) {
    const path = readPath(text, direct[0].length);
    return path && path.length > 0 ? { type: 'json', label: pathLabel(path), path } : raw;
  }

  if (text.startsWith('$(')) {
    const quote = text.charAt(2);
    if (quote !== "'" && quote !== '"') return raw;
    const name = readString(text, 2);
    if (!name || text.charAt(name.end) !== ')') return raw;
    const rest = /^\.item\.json|^\.first\(\)\.json|^\.last\(\)\.json/.exec(text.slice(name.end + 1));
    if (!rest) return raw;
    const path = readPath(text, name.end + 1 + rest[0].length);
    return path && path.length > 0 ? { type: 'node', label: `${name.value} > ${pathLabel(path)}`, path, node: name.value } : raw;
  }
  return raw;
}

/**
 * Where the `{{ ... }}` block that starts at `start` ends: the index after its `}}`, or -1 when it is never closed.
 * Quoted strings and nested braces inside it (an object literal) do not end it.
 */
function blockEnd(text: string, start: number): number {
  let index = start + 2;
  let depth = 0;
  while (index < text.length) {
    const char = text.charAt(index);
    if (char === "'" || char === '"' || char === '`') {
      const read = readString(text, index);
      if (!read) return -1;
      index = read.end;
    } else if (char === '{') {
      depth += 1;
      index += 1;
    } else if (char === '}') {
      if (depth === 0 && text.charAt(index + 1) === '}') return index + 2;
      depth = Math.max(0, depth - 1);
      index += 1;
    } else {
      index += 1;
    }
  }
  return -1;
}

/** Splits stored text into segments. For text that is not an expression, one text segment holds all of it. */
export function textToChips(stored: string): ParsedExpression {
  if (!stored.startsWith('=')) return { expression: false, segments: stored === '' ? [] : [{ kind: 'text', raw: stored }] };
  const body = stored.slice(1);
  const segments: Segment[] = [];
  let plain = '';
  let index = 0;
  while (index < body.length) {
    if (body.startsWith('{{', index)) {
      const end = blockEnd(body, index);
      if (end > 0) {
        if (plain !== '') segments.push({ kind: 'text', raw: plain });
        plain = '';
        const raw = body.slice(index, end);
        segments.push({ kind: 'expr', raw, chip: classify(raw.slice(2, -2)) });
        index = end;
        continue;
      }
    }
    plain += body.charAt(index);
    index += 1;
  }
  if (plain !== '') segments.push({ kind: 'text', raw: plain });
  return { expression: true, segments };
}

/** The stored text a list of segments makes. The exact inverse of `textToChips`. */
export function chipsToText(parsed: ParsedExpression): string {
  return `${parsed.expression ? '=' : ''}${parsed.segments.map((segment) => segment.raw).join('')}`;
}

const SIMPLE_NAME = /^[A-Za-z_$][A-Za-z0-9_$]*$/;

/** The accessors for a path: `.name` for names that are identifiers, `['...']` for the rest, `[0]` for positions. */
export function accessors(path: readonly (string | number)[]): string {
  return path
    .map((step) => (typeof step === 'number' ? `[${String(step)}]` : SIMPLE_NAME.test(step) ? `.${step}` : `['${step.replace(/\\/g, '\\\\').replace(/'/g, "\\'")}']`))
    .join('');
}

export interface FieldRef {
  /** The steps into the data of the item. */
  readonly path: readonly (string | number)[];
  /** The step the field comes from. */
  readonly node: string;
}

/**
 * The block that reads a field (SPEC-014 R-ED-EXPR-01): `{{ $json.a }}` when the field comes from the only step that
 * feeds this one, `{{ $('Step').item.json.a }}` otherwise.
 */
export function fieldExpression(field: FieldRef, direct: boolean): string {
  const root = direct ? '$json' : `$('${field.node.replace(/\\/g, '\\\\').replace(/'/g, "\\'")}').item.json`;
  return `{{ ${root}${accessors(field.path)} }}`;
}

/** The stored text with a field added after what it holds. A fixed text first becomes an expression that keeps its text. */
export function appendField(stored: string, block: string): string {
  const body = stored.startsWith('=') ? stored.slice(1) : stored;
  return `=${body}${block}`;
}

/** The stored text without the segment at `index`. */
export function removeSegment(parsed: ParsedExpression, index: number): string {
  return chipsToText({ ...parsed, segments: parsed.segments.filter((_, position) => position !== index) });
}
