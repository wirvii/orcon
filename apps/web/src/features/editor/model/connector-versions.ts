// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { FlowNode } from './flow-document';

/**
 * The connector versions installed on the instance, until the node-type view of the API (SPEC-005
 * A10, cut ED4a) provides them. Each flow node pins `connector { version, digest }` (ADR-0034 §1);
 * the editor offers the newest installed version and its changelog (SPEC-003 R-VER-03). When the
 * API view arrives it replaces this table; the code that reads it does not change.
 */

export type ChangeKind = 'added' | 'changed' | 'removed';

export interface ConnectorChange {
  readonly kind: ChangeKind;
  readonly text: { readonly en: string; readonly es: string };
  /** The change can alter what the node does: a person should check it after updating. */
  readonly review?: boolean;
}

export interface ConnectorRelease {
  readonly version: string;
  readonly digest: string;
  readonly changes: readonly ConnectorChange[];
}

const digest = (seed: string) => `sha256:${seed.repeat(64).slice(0, 64)}`;

export const INSTALLED_CONNECTORS: Readonly<Record<string, readonly ConnectorRelease[]>> = {
  'orcon.http@1': [
    { version: '1.0.0', digest: digest('a1'), changes: [] },
    {
      version: '1.1.0',
      digest: digest('b2'),
      changes: [{ kind: 'added', text: { en: 'New option "Response format" (default: detect it from the response).', es: 'Opción nueva "Formato de respuesta" (por defecto: detectarlo en la respuesta).' } }],
    },
    {
      version: '1.2.0',
      digest: digest('c3'),
      changes: [
        { kind: 'changed', review: true, text: { en: 'Redirects to another host are no longer followed unless the request allows it.', es: 'Las redirecciones a otro servidor ya no se siguen salvo que la petición lo permita.' } },
        { kind: 'added', text: { en: 'Error messages now include the status text of the response.', es: 'Los mensajes de error incluyen ahora el texto de estado de la respuesta.' } },
      ],
    },
  ],
};

/** `1.2.0` as numbers; a missing or malformed part counts as 0. */
function parts(version: string): [number, number, number] {
  const [major = '0', minor = '0', patch = '0'] = version.split('.');
  return [Number.parseInt(major, 10) || 0, Number.parseInt(minor, 10) || 0, Number.parseInt(patch, 10) || 0];
}

/** Negative when `a` is older than `b`, positive when newer, 0 when equal. */
export function compareVersions(a: string, b: string): number {
  const left = parts(a);
  const right = parts(b);
  for (let index = 0; index < 3; index += 1) {
    const difference = (left[index] ?? 0) - (right[index] ?? 0);
    if (difference !== 0) return difference;
  }
  return 0;
}

/** The connector pin of a node (`connector { version, digest }`), if it has one. */
export function pinOf(node: FlowNode): { version: string; digest: string } | undefined {
  const block = node['connector'];
  if (typeof block !== 'object' || block === null) return undefined;
  const { version, digest: hash } = block as Record<string, unknown>;
  return typeof version === 'string' ? { version, digest: typeof hash === 'string' ? hash : '' } : undefined;
}

export interface AvailableUpdate {
  readonly from: string;
  readonly latest: ConnectorRelease;
  /** Every release after the pinned one, oldest first; their changes make the changelog. */
  readonly releases: readonly ConnectorRelease[];
  /** Some change can alter what the node does. When false the whole step is mechanical. */
  readonly needsReview: boolean;
}

/** The update the node can take, when it pins a version and a newer one is installed. */
export function availableUpdate(node: FlowNode): AvailableUpdate | undefined {
  const pin = pinOf(node);
  const installed = INSTALLED_CONNECTORS[node.type];
  if (!pin || !installed) return undefined;
  const releases = installed.filter((release) => compareVersions(release.version, pin.version) > 0).sort((a, b) => compareVersions(a.version, b.version));
  const latest = releases[releases.length - 1];
  if (!latest) return undefined;
  return { from: pin.version, latest, releases, needsReview: releases.some((release) => release.changes.some((change) => change.review === true)) };
}
