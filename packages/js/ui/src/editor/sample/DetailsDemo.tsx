// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { useState } from 'react';

import type { GraphNode } from '../../canvas/model';
import { useMediaQuery } from '../../canvas/useMediaQuery';
import { DataPane } from '../data-pane/DataPane';
import { orderItems } from '../fixtures';
import { NodeDetailsView } from '../node-details-view/NodeDetailsView';
import type { PaneData } from '../model';

import { AgentSample, DocsSample, NodeStatusAlert, ParametersSample, SettingsSample } from './ParametersSample';

export interface DetailsDemoProps {
  node: GraphNode;
  layout?: 'dialog' | 'docked' | 'auto' | undefined;
  input?: PaneData | undefined;
  output?: PaneData | undefined;
}

/** The node details view with sample panes, for stories. In `auto` it follows the lg breakpoint like the editor does. */
export function DetailsDemo({ node, layout = 'auto', input = { items: orderItems, from: 'Webhook' }, output }: DetailsDemoProps) {
  const [open, setOpen] = useState(true);
  const wide = useMediaQuery('(min-width: 64rem)');
  const resolved = layout === 'auto' ? (wide ? 'dialog' : 'docked') : layout;
  const result = output ?? { items: orderItems, from: node.name, run: { index: 1, total: 1 } };
  return (
    <NodeDetailsView
      node={node}
      open={open}
      onOpenChange={setOpen}
      layout={resolved}
      input={<DataPane mode="input" data={input} onExecute={() => undefined} className="flex-1" />}
      output={<DataPane mode="output" data={result} onPinChange={() => undefined} onEdit={() => undefined} onExecute={() => undefined} className="flex-1" />}
      tabs={{ parameters: <ParametersSample node={node} />, settings: <SettingsSample />, docs: <DocsSample />, agent: node.kind === 'agent' ? <AgentSample /> : undefined }}
      alert={<NodeStatusAlert node={node} />}
      onExecute={() => undefined}
      onRename={() => undefined}
    />
  );
}
