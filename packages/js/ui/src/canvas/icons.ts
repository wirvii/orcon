// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import {
  Bot,
  CircleHelp,
  Clock,
  Code,
  Database,
  FileText,
  Filter,
  GitBranch,
  Globe,
  Mail,
  Merge,
  MessageSquare,
  Play,
  Repeat,
  Split,
  Timer,
  Webhook,
  Zap,
  type LucideIcon,
} from 'lucide-react';

import type { GraphNode, LogicType, NodeIconName } from './model';

/** Built-in node icons. They are Lucide icons; connector nodes show their logo instead. */
export const nodeIcons: Record<NodeIconName, LucideIcon> = {
  webhook: Webhook,
  globe: Globe,
  mail: Mail,
  message: MessageSquare,
  database: Database,
  clock: Clock,
  file: FileText,
  code: Code,
  play: Play,
  bot: Bot,
  zap: Zap,
  help: CircleHelp,
};

export const logicIcons: Record<LogicType, LucideIcon> = {
  if: GitBranch,
  switch: Split,
  filter: Filter,
  merge: Merge,
  loop: Repeat,
  wait: Timer,
};

/** The Lucide icon of a node that has no connector logo. */
export function iconFor(node: Pick<GraphNode, 'kind' | 'logicType' | 'icon'>): LucideIcon {
  if (node.kind === 'placeholder') return CircleHelp;
  if (node.kind === 'logic') return logicIcons[node.logicType ?? 'if'];
  if (node.icon) return nodeIcons[node.icon];
  if (node.kind === 'trigger') return Zap;
  if (node.kind === 'agent') return Bot;
  return Play;
}
