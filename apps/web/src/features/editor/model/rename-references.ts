// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { FlowDocument } from './flow-document';

/**
 * Renaming a node rewrites the references to it in expressions (SPEC-014 R-ED-CAN-04): the string
 * literal of `$('Old name')` and `$node["Old name"]`, matched exactly, quote style kept. The scan
 * reads string literals with their escapes, so a name that merely appears inside another literal
 * or in a longer name is never touched.
 */

const OPENERS = ['$(', '$node['] as const;
const CLOSERS: Record<(typeof OPENERS)[number], string> = { '$(': ')', '$node[': ']' };

interface Literal {
  /** Index of the opening quote. */
  start: number;
  /** Index after the closing quote. */
  end: number;
  quote: string;
  value: string;
}

/** Reads a quoted string literal that starts at `index` (a quote), with backslash escapes. */
function readLiteral(text: string, index: number): Literal | undefined {
  const quote = text[index];
  if (quote !== '"' && quote !== "'") return undefined;
  let value = '';
  let cursor = index + 1;
  while (cursor < text.length) {
    const char = text[cursor];
    if (char === '\\') {
      const escaped = text[cursor + 1];
      if (escaped === undefined) return undefined;
      value += escaped;
      cursor += 2;
    } else if (char === quote) {
      return { start: index, end: cursor + 1, quote, value };
    } else {
      value += char ?? '';
      cursor += 1;
    }
  }
  return undefined;
}

const quoteAs = (value: string, quote: string) => `${quote}${value.replaceAll('\\', '\\\\').replaceAll(quote, `\\${quote}`)}${quote}`;

function skipSpaces(text: string, index: number): number {
  let cursor = index;
  while (cursor < text.length && /\s/.test(text[cursor] ?? '')) cursor += 1;
  return cursor;
}

/** Rewrites the references in one expression text. */
export function renameInExpression(text: string, from: string, to: string): { text: string; count: number } {
  let out = '';
  let count = 0;
  let cursor = 0;
  while (cursor < text.length) {
    const opener = OPENERS.find((candidate) => text.startsWith(candidate, cursor));
    if (!opener) {
      out += text[cursor] ?? '';
      cursor += 1;
      continue;
    }
    const literalAt = skipSpaces(text, cursor + opener.length);
    const literal = readLiteral(text, literalAt);
    const closeAt = literal ? skipSpaces(text, literal.end) : -1;
    if (literal?.value === from && text.startsWith(CLOSERS[opener], closeAt)) {
      out += text.slice(cursor, literalAt) + quoteAs(to, literal.quote) + text.slice(literal.end, closeAt + 1);
      cursor = closeAt + 1;
      count += 1;
    } else {
      out += opener;
      cursor += opener.length;
    }
  }
  return { text: out, count };
}

/** Rewrites every `=`-prefixed string below `value`. Returns the number of references changed. */
function rewriteValue(value: unknown, from: string, to: string, holder: Record<string, unknown> | unknown[], key: string | number): number {
  if (typeof value === 'string') {
    if (!value.startsWith('=')) return 0;
    const result = renameInExpression(value, from, to);
    if (result.count > 0) (holder as Record<string | number, unknown>)[key] = result.text;
    return result.count;
  }
  let total = 0;
  if (Array.isArray(value)) {
    value.forEach((item, index) => {
      total += rewriteValue(item, from, to, value, index);
    });
  } else if (typeof value === 'object' && value !== null) {
    const record = value as Record<string, unknown>;
    for (const name of Object.keys(record)) total += rewriteValue(record[name], from, to, record, name);
  }
  return total;
}

/**
 * Rewrites, in place, the references to node `from` in the parameters and other blocks of every
 * node and in the edge conditions of the document. Returns how many references changed.
 */
export function renameReferences(document: FlowDocument, from: string, to: string): number {
  if (from === to) return 0;
  let total = 0;
  for (const node of document.nodes) {
    for (const key of Object.keys(node)) {
      if (key === 'id' || key === 'name' || key === 'element' || key === 'type') continue;
      total += rewriteValue(node[key], from, to, node, key);
    }
  }
  for (const edge of document.edges) {
    if (edge.condition === undefined) continue;
    const result = renameInExpression(edge.condition, from, to);
    if (result.count > 0) {
      edge.condition = result.text;
      total += result.count;
    }
  }
  return total;
}
