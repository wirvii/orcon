// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * The allowed hosts of a credential: the sites its secret may be sent to (SPEC-004 R-CRED-03, ADR-0043 §4).
 * The server keeps a list of at least one and at most 20 entries, each a domain name such as `api.example.com`
 * or one wildcard label over a domain such as `*.example.com`; schemes, ports, paths and IPv6 literals are
 * refused. The page applies the same rules before it sends, so the person sees the problem next to the field,
 * and turns what people really paste (a whole address) into the host.
 */

/** The most hosts one credential holds (the server's own bound). */
export const MAX_ALLOWED_HOSTS = 20;

const MAX_HOST_LENGTH = 253;
const LABEL = /^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$/;

export type HostResult =
  | { readonly ok: true; readonly host: string }
  | {
      readonly ok: false;
      readonly reason: 'empty' | 'invalid' | 'wildcard-address';
    };

/** The host a pasted address or name stands for, or why it cannot be one. */
export function parseAllowedHost(input: string): HostResult {
  let text = input.trim().toLowerCase();
  if (text === '') return { ok: false, reason: 'empty' };

  // A whole address (`https://api.example.com/v1`, `api.example.com:443/x`) stands for its host.
  const wildcard = text.startsWith('*.');
  if (wildcard) text = text.slice(2);
  if (/[/:?#@\s]/.test(text)) {
    try {
      text = new URL(text.includes('://') ? text : `https://${text}`).hostname;
    } catch {
      return { ok: false, reason: 'invalid' };
    }
  }
  const host = text.endsWith('.') ? text.slice(0, -1) : text;
  const full = wildcard ? `*.${host}` : host;
  if (host === '' || full.length > MAX_HOST_LENGTH) return { ok: false, reason: 'invalid' };

  const labels = host.split('.');
  if (labels.length < 2 || !labels.every((label) => LABEL.test(label))) return { ok: false, reason: 'invalid' };
  // A wildcard over an address, or over a tail of digits that only looks like part of one, matches no real site.
  if (wildcard && /^[0-9]+$/.test(labels[labels.length - 1] ?? '')) return { ok: false, reason: 'wildcard-address' };
  return { ok: true, host: full };
}

/** Adds a host to a list without repeating one. */
export function withHost(hosts: readonly string[], host: string): readonly string[] {
  return hosts.includes(host) ? hosts : [...hosts, host];
}
