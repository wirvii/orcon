// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest';

import { appendField, chipsToText, fieldExpression, removeSegment, textToChips, type Segment } from './expression-chips';

const exprs = (stored: string): Extract<Segment, { kind: 'expr' }>[] =>
  textToChips(stored).segments.filter((segment): segment is Extract<Segment, { kind: 'expr' }> => segment.kind === 'expr');

test('expressionChips renders a json path as a readable chip', () => {
  const [chip] = exprs('={{ $json.customer.phone }}');
  expect(chip?.chip).toEqual({ type: 'json', label: 'customer.phone', path: ['customer', 'phone'] });
  // Positions and quoted names are part of the path too.
  expect(exprs("={{ $json.items[0]['unit price'] }}")[0]?.chip).toEqual({ type: 'json', label: 'items[0].unit price', path: ['items', 0, 'unit price'] });
  // `$input.item.json` is the same data as `$json`.
  expect(exprs('={{ $input.item.json.name }}')[0]?.chip).toMatchObject({ type: 'json', label: 'name' });
});

test('expressionChips renders a node reference as node and path', () => {
  expect(exprs("={{ $('Webhook').item.json.body.id }}")[0]?.chip).toEqual({ type: 'node', node: 'Webhook', label: 'Webhook > body.id', path: ['body', 'id'] });
  expect(exprs('={{ $("Ask the model").item.json.answer }}')[0]?.chip).toMatchObject({ type: 'node', node: 'Ask the model', label: 'Ask the model > answer' });
});

test('expressionChips keeps complex expressions as raw text inside a chip', () => {
  for (const source of ['$json.total > 0', '$json.a + $json.b', '$json.name.toUpperCase()', '$json.ok ? "yes" : "no"', '$json', '$vars.region']) {
    const [chip] = exprs(`={{ ${source} }}`);
    expect(chip?.chip).toEqual({ type: 'raw', label: source });
  }
  // Text around a block stays text.
  const parsed = textToChips('=Hello {{ $json.name }}, you owe {{ $json.total * 2 }}.');
  expect(parsed.segments.map((segment) => segment.kind)).toEqual(['text', 'expr', 'text', 'expr', 'text']);
});

/** A small deterministic generator, so a failure can be reproduced: the same seed always makes the same inputs. */
function random(seed: number): () => number {
  let state = seed;
  return () => {
    state = (state * 1664525 + 1013904223) % 4294967296;
    return state / 4294967296;
  };
}

test('expressionChips round trip text to chips to text is byte identical for any input', () => {
  const alphabet = ['{', '{', '}', '}', '{{', '}}', '=', ' ', "'", '"', '`', '\\', '$json', '.a', "['b']", '[0]', "$('N')", '.item.json', 'x', '\n', 'é', '🙂', '$', '(', ')'];
  const next = random(20261015);
  for (let round = 0; round < 3000; round += 1) {
    const length = Math.floor(next() * 14);
    let text = next() < 0.7 ? '=' : '';
    for (let index = 0; index < length; index += 1) text += alphabet[Math.floor(next() * alphabet.length)] ?? '';
    expect(chipsToText(textToChips(text))).toBe(text);
  }
  // The edge cases a generator may not reach.
  for (const text of ['', '=', '={{', '={{}}', '={{ }}', '={{ unterminated', "={{ '}}' }}", '={{ {a:1} }}', '=}}{{', 'plain text', '={{ $json.a }}{{ $json.b }}']) {
    expect(chipsToText(textToChips(text))).toBe(text);
  }
});

test('expressionChips accepts pasted n8n expressions', () => {
  // n8n writes the same blocks with its own spacing; the chip is the same.
  expect(exprs('={{$json.customer.phone}}')[0]?.chip).toMatchObject({ type: 'json', label: 'customer.phone' });
  expect(exprs("={{ $('HTTP Request').item.json.status }}")[0]?.chip).toMatchObject({ type: 'node', label: 'HTTP Request > status' });
  expect(exprs("={{ $('HTTP Request').first().json.status }}")[0]?.chip).toMatchObject({ type: 'node', node: 'HTTP Request' });
  // What was pasted is what is stored.
  const pasted = '={{ $json.a }} and {{ $("Webhook").item.json.b }}';
  expect(chipsToText(textToChips(pasted))).toBe(pasted);
});

test('expressionChips never changes the stored equals-prefixed string', () => {
  const stored = '=Hi {{ $json.name }}!';
  const parsed = textToChips(stored);
  expect(parsed.expression).toBe(true);
  expect(chipsToText(parsed)).toBe(stored);
  // Fixed text has no prefix and gets none.
  expect(chipsToText(textToChips('Hi there'))).toBe('Hi there');
  // Adding a field to fixed text makes it an expression that keeps the text; removing the chip leaves the text.
  const appended = appendField('Hi ', fieldExpression({ node: 'Webhook', path: ['name'] }, true));
  expect(appended).toBe('=Hi {{ $json.name }}');
  expect(removeSegment(textToChips(appended), 1)).toBe('=Hi ');
});

test('expressionChips writes a field the way the guide says: direct input, other step, odd names', () => {
  expect(fieldExpression({ node: 'Webhook', path: ['customer', 'phone'] }, true)).toBe('{{ $json.customer.phone }}');
  expect(fieldExpression({ node: 'Webhook', path: ['customer', 'phone'] }, false)).toBe("{{ $('Webhook').item.json.customer.phone }}");
  expect(fieldExpression({ node: "It's", path: ['unit price', 0, 'a'] }, false)).toBe("{{ $('It\\'s').item.json['unit price'][0].a }}");
  // What is written reads back as the chip it was made from.
  expect(exprs(`=${fieldExpression({ node: "It's", path: ['unit price', 0, 'a'] }, false)}`)[0]?.chip).toMatchObject({ type: 'node', node: "It's", path: ['unit price', 0, 'a'] });
});
