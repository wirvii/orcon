// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { CatalogItem } from '@orcon/ui';
import { useCallback, useMemo } from 'react';
import { useIntl, type MessageDescriptor } from 'react-intl';

import { brandIconFor } from '../model/brand-marks';
import { ADDED_ID_PREFIX, NODE_TYPES, typeDefOf, type NodeGroup } from '../model/node-types';
import type { FlowNode } from '../model/flow-document';

import { editorMessages, nodeTypeMessages } from './messages';

const names: Record<string, MessageDescriptor> = {
  webhook: nodeTypeMessages.webhook,
  manual: nodeTypeMessages.manual,
  http: nodeTypeMessages.http,
  set: nodeTypeMessages.set,
  noop: nodeTypeMessages.noop,
  if: nodeTypeMessages.if,
  merge: nodeTypeMessages.merge,
  loop: nodeTypeMessages.loop,
  call: nodeTypeMessages.call,
  end: nodeTypeMessages.end,
};

const descriptions: Record<string, MessageDescriptor> = {
  webhook: nodeTypeMessages.webhookText,
  manual: nodeTypeMessages.manualText,
  http: nodeTypeMessages.httpText,
  set: nodeTypeMessages.setText,
  noop: nodeTypeMessages.noopText,
  if: nodeTypeMessages.ifText,
  merge: nodeTypeMessages.mergeText,
  loop: nodeTypeMessages.loopText,
  call: nodeTypeMessages.callText,
  end: nodeTypeMessages.endText,
};

const groups: Record<NodeGroup, MessageDescriptor> = {
  triggers: editorMessages.catalogTriggers,
  apps: editorMessages.catalogApps,
  data: editorMessages.catalogData,
  flow: editorMessages.catalogFlow,
  subflows: editorMessages.catalogSubflows,
};

/**
 * The nodes the palette offers, in the person's language. A node added from the palette starts with
 * the name shown here; when the flow already has a node of that name the document gives the new one
 * a free name ("HTTP Request 2") and the editor tells the canvas (`EditReport.adjustedNames`).
 */
export function useNodeCatalog(): CatalogItem[] {
  const intl = useIntl();
  return useMemo(() => {
    return NODE_TYPES.map((def): CatalogItem => {
      const name = names[def.slug];
      const text = descriptions[def.slug];
      return {
        id: `${ADDED_ID_PREFIX}${def.slug}`,
        kind: def.kind,
        logicType: def.logic,
        name: name ? intl.formatMessage(name) : def.slug,
        subtitle: text ? intl.formatMessage(text) : undefined,
        group: intl.formatMessage(groups[def.group]),
        icon: def.icon,
        connector: brandIconFor(def.type),
        keywords: def.aliases,
        inputs: def.kind === 'trigger' ? [] : [{ id: 'main' }],
        outputs: def.outputs.map((port) => ({ id: port.id, label: port.label })),
      };
    });
  }, [intl]);
}

/** The line under a node whose parameters give none: the description of its type. */
export function useDescribeNode(): (node: FlowNode) => string | undefined {
  const intl = useIntl();
  return useCallback(
    (node) => {
      const slug = typeDefOf(node)?.slug;
      const text = slug ? descriptions[slug] : undefined;
      return text ? intl.formatMessage(text) : undefined;
    },
    [intl],
  );
}
