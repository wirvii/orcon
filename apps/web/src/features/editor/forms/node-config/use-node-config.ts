// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { useCallback, useMemo, useRef } from 'react';
import { useIntl } from 'react-intl';

import type { FlowNode } from '../../model/flow-document';
import { catalogText, localeOf, type NodeManifest } from '../../model/node-manifests';
import { useEditorStore, useEditorStoreApi } from '../../store/editor-context';
import type { NodeBlock } from '../../model/document-ops';

/** The node with this id, as the store holds it now; the form re-renders only when this node changes. */
export function useNode(nodeId: string): FlowNode | undefined {
  return useEditorStore((state) => state.document.nodes.find((node) => node.id === nodeId));
}

/** `t('param.url.label')`: a text of the manifest's language catalog in the language of the page. */
export function useCatalog(manifest: Pick<NodeManifest, 'catalog'> | undefined): (key: string) => string {
  const { locale } = useIntl();
  const language = localeOf(locale);
  return useCallback((key) => (manifest ? catalogText(manifest, language, key) : key), [manifest, language]);
}

let tokens = 0;
/** A new history group: edits that carry the same group in a row are one undo step. */
const newGroup = (label: string): string => {
  tokens += 1;
  return `${label}#${String(tokens)}`;
};

/**
 * The edits one node's form makes, as store actions. `useFieldGroup(label)` gives a field the group
 * of its current focus: everything typed while the field keeps focus is one undo step (SPEC-014
 * R-ED-UNDO-01), and focusing the field again starts a new step.
 */
export function useNodeActions(nodeId: string) {
  const api = useEditorStoreApi();
  return useMemo(
    () => ({
      setParameter: (name: string, value: unknown, group?: string) => {
        api.getState().setParameter(nodeId, name, value, group);
      },
      setBlock: (block: NodeBlock, value: unknown, group?: string) => {
        api.getState().setNodeBlock(nodeId, block, value, group);
      },
      setCredential: (slot: string, name: string | undefined) => {
        api.getState().setCredential(nodeId, slot, name);
      },
      updateConnector: (version: string, digest: string) => {
        api.getState().updateConnector(nodeId, version, digest);
      },
    }),
    [api, nodeId],
  );
}

export function useFieldGroup(label: string): { group: () => string; renew: () => void } {
  const current = useRef<string | undefined>(undefined);
  return useMemo(
    () => ({
      group: () => (current.current ??= newGroup(label)),
      renew: () => {
        current.current = newGroup(label);
      },
    }),
    [label],
  );
}
