// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { ReactNode } from 'react';

// The Markdown subset of a sticky note: paragraphs, "- " lists, **bold**, *italic* and `code`.
// Raw HTML is never interpreted: every piece of text is rendered as text.
const inline = /(\*\*[^*]+\*\*|\*[^*]+\*|`[^`]+`)/g;

function renderInline(text: string, keyBase: string): ReactNode[] {
  return text.split(inline).map((part, index) => {
    const key = `${keyBase}-${index}`;
    if (part.startsWith('**') && part.endsWith('**') && part.length > 4) return <strong key={key}>{part.slice(2, -2)}</strong>;
    if (part.startsWith('`') && part.endsWith('`') && part.length > 2) return <code key={key} className="font-mono">{part.slice(1, -1)}</code>;
    if (part.startsWith('*') && part.endsWith('*') && part.length > 2) return <em key={key}>{part.slice(1, -1)}</em>;
    return part;
  });
}

/** Renders the Markdown subset of a sticky note as React elements. */
export function NoteMarkdown({ text }: { text: string }) {
  const lines = text.split('\n');
  const blocks: ReactNode[] = [];
  let list: string[] = [];
  const flush = () => {
    if (list.length > 0) {
      const items = list;
      blocks.push(
        <ul key={`list-${blocks.length}`} className="list-disc ps-4">
          {items.map((item, index) => (
            <li key={`${item}-${index}`}>{renderInline(item, `li-${blocks.length}-${index}`)}</li>
          ))}
        </ul>,
      );
      list = [];
    }
  };
  lines.forEach((line, index) => {
    const bullet = /^\s*[-*]\s+(.*)$/.exec(line);
    if (bullet) {
      list.push(bullet[1] ?? '');
      return;
    }
    flush();
    if (line.trim() === '') return;
    blocks.push(<p key={`p-${index}`}>{renderInline(line, `p-${index}`)}</p>);
  });
  flush();
  return <>{blocks}</>;
}
