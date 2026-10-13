// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from 'vitest';

import { exampleFlowDocument } from './example-flow';
import { uniqueName } from './names';
import { renameInExpression, renameReferences } from './rename-references';

describe('renaming a node in an expression', () => {
  it('rewrites both reference forms and keeps the quote style', () => {
    expect(renameInExpression(`={{ $('Old').item.json.id + $("Old").first().x + $node["Old"].json + $node['Old'].json }}`, 'Old', 'New')).toEqual({
      text: `={{ $('New').item.json.id + $("New").first().x + $node["New"].json + $node['New'].json }}`,
      count: 4,
    });
  });

  it('matches the name exactly: longer names, other literals and plain strings are left alone', () => {
    const text = `={{ $('Old name').x + $('Older').y + "see $('Old')" + $(Old) + $( 'Old' ) }}`;
    const result = renameInExpression(text, 'Old', 'New');
    expect(result.count).toBe(2);
    expect(result.text).toBe(`={{ $('Old name').x + $('Older').y + "see $('New')" + $(Old) + $( 'New' ) }}`);
  });

  it('reads escapes in the literal and writes them back', () => {
    expect(renameInExpression(`$('It\\'s')`, `It's`, `He said "hi"`)).toEqual({ text: `$('He said "hi"')`, count: 1 });
    expect(renameInExpression(`$("a\\\\b")`, 'a\\b', 'c')).toEqual({ text: `$("c")`, count: 1 });
  });

  it('ignores an unterminated literal and a reference whose bracket never closes', () => {
    expect(renameInExpression(`$('Old`, 'Old', 'New')).toEqual({ text: `$('Old`, count: 0 });
    expect(renameInExpression(`$('Old' + 1`, 'Old', 'New').count).toBe(0);
    expect(renameInExpression(`$node["Old"x`, 'Old', 'New').count).toBe(0);
    expect(renameInExpression(`$('a\\`, 'a', 'b').count).toBe(0);
  });
});

describe('renaming a node in a document', () => {
  it('rewrites expression parameters and edge conditions, never plain strings, and reports how many', () => {
    const document = exampleFlowDocument();
    const slack = document.nodes.find((node) => node.id === 'slack');
    const done = document.nodes.find((node) => node.id === 'done');
    if (!slack || !done) throw new Error('missing');
    done.parameters = { note: `$('Webhook') stays as typed`, list: [`=$('Webhook').a`, 5, { deep: `={{ $('Webhook').b }}` }] };
    const edge = document.edges[0];
    if (edge) edge.condition = `$('Webhook').item.json.ok`;

    const count = renameReferences(document, 'Webhook', 'Order intake');

    expect(count).toBe(4);
    expect(slack.parameters['body']).toContain(`$('Order intake').item.json.id`);
    expect(done.parameters['note']).toBe(`$('Webhook') stays as typed`);
    expect(done.parameters['list']).toEqual([`=$('Order intake').a`, 5, { deep: `={{ $('Order intake').b }}` }]);
    expect(document.edges[0]?.condition).toBe(`$('Order intake').item.json.ok`);
  });

  it('does nothing when the name does not change', () => {
    expect(renameReferences(exampleFlowDocument(), 'Webhook', 'Webhook')).toBe(0);
  });
});

describe('unique node names', () => {
  it('keeps a free name and numbers a taken one from 2', () => {
    expect(uniqueName('Send email', new Set(['Other']))).toBe('Send email');
    expect(uniqueName('Send email', new Set(['Send email']))).toBe('Send email 2');
    expect(uniqueName('Send email', new Set(['Send email', 'Send email 2']))).toBe('Send email 3');
    expect(uniqueName('Send email 2', new Set(['Send email', 'Send email 2']))).toBe('Send email 3');
    expect(uniqueName('x'.repeat(128), new Set(['x'.repeat(128)]))).toHaveLength(128);
  });
});
