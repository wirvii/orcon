// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/** The format tag of the text the editor puts on the clipboard (SPEC-014 R-ED-CLIP-01, orcon-web ST-8). */
export const FLOW_FRAGMENT_FORMAT = 'orcon/flow-fragment@1';

/** What a pasted text holds, as far as its shape says. */
export type ClipboardFormat = 'orcon-fragment' | 'n8n-workflow' | 'unknown';

/** The most text the editor reads from the clipboard (SPEC-014 R-ED-SEC-02). */
export const MAX_CLIPBOARD_BYTES = 2 * 1024 * 1024;

const isRecord = (value: unknown): value is Record<string, unknown> => typeof value === 'object' && value !== null && !Array.isArray(value);

/**
 * Tells an Orcon fragment from an n8n workflow by shape (SPEC-014 section 11): a fragment carries
 * the `orcon/flow-fragment@1` tag; an n8n workflow has a `nodes` list whose members have a `type`
 * and a `connections` object. Anything else is `unknown`. Detection only looks at shape; the
 * content is validated by the schema of the format it names.
 */
export function detectClipboardFormat(value: unknown): ClipboardFormat {
  if (!isRecord(value)) return 'unknown';
  if (value['format'] === FLOW_FRAGMENT_FORMAT) return 'orcon-fragment';
  const nodes = value['nodes'];
  if (Array.isArray(nodes) && nodes.length > 0 && isRecord(value['connections']) && nodes.every((node) => isRecord(node) && typeof node['type'] === 'string')) {
    return 'n8n-workflow';
  }
  return 'unknown';
}
