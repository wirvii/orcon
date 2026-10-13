// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Zap } from 'lucide-react';
import { useEffect, useRef, useState, type CSSProperties, type KeyboardEvent, type ReactNode } from 'react';
import { useIntl } from 'react-intl';

import { cn } from '../../lib/cn';
import { AiBlock } from '../ai-block/AiBlock';
import type { CanvasNodeActions, DetailLevel } from '../context';
import { canvasMessages } from '../messages';
import { metrics, nodeSize, portOffsets, type GraphNode } from '../model';
import { NodeGlyph } from '../NodeGlyph';
import { NodePort } from '../node-port/NodePort';
import { NodeStatusBadge } from '../node-status-badge/NodeStatusBadge';
import { NodeStatusChip, type NodeStatusChipProps } from '../node-status-chip/NodeStatusChip';
import { NodeToolbar } from '../node-toolbar/NodeToolbar';
import { nodeAriaLabel, portLabel } from '../nodeLabel';
import { OutputStub } from '../output-stub/OutputStub';
import { PinnedBadge } from '../pinned-badge/PinnedBadge';
import { diffFamily, familyClasses, familyOf, nodeVisual } from '../state';

/** Radius class of the tile (and of the focus ring of its React Flow wrapper). A trigger also needs {@link tileRadiusStyle}. */
export const tileRadius = (node: Pick<GraphNode, 'kind'>) => (node.kind === 'trigger' ? '' : 'rounded-2xl');

/**
 * The trigger is a half circle on the left and `radius.2xl` on the right. The two radii cannot be
 * classes: a half circle larger than the tile would shrink the right corners too.
 */
export const tileRadiusStyle = (node: Pick<GraphNode, 'kind'>): CSSProperties | undefined =>
  node.kind === 'trigger'
    ? { borderRadius: 'calc(var(--orcon-canvas-node-size) / 2) var(--orcon-radius-2xl) var(--orcon-radius-2xl) calc(var(--orcon-canvas-node-size) / 2)' }
    : undefined;

/** Distance from the bottom of the agent tile to the top of its configuration blocks. */
const AI_BLOCK_OFFSET = 84;

export interface CanvasNodeProps {
  node: GraphNode;
  selected?: boolean;
  /** Ports are live React Flow handles. True inside the canvas. */
  live?: boolean;
  /** The canvas can be edited: toolbar, plus buttons and stubs are offered. False in read-only mode. */
  editable?: boolean;
  /** Reachable with Tab: only the node that holds the roving tab stop. */
  tabbable?: boolean;
  /** The node has keyboard focus. Shows the toolbar like hover does. */
  focused?: boolean;
  /** Show the toolbar whatever the pointer does (stories). */
  forceToolbar?: boolean;
  detail?: DetailLevel;
  /** Counter-scale of the label at low zoom. */
  labelScale?: number;
  /** Outputs that have no connection yet: they show the dashed stub. */
  dangling?: readonly string[];
  /** Inputs that cannot receive the connection being dragged, with the reason. */
  invalidInputs?: Readonly<Record<string, string>>;
  /** The name is being edited in place. */
  renaming?: boolean;
  /** Give the tile its own accessible name. The canvas names the React Flow node element instead. */
  labelled?: boolean;
  actions?: Partial<CanvasNodeActions>;
  className?: string;
}

/**
 * A node of the flow in style A: a 100 px square tile with a large icon, a name and an operation
 * under it, status shown by a corner badge, a chip, the border and the incoming connection, ports on
 * the sides and, on hover, focus or selection, a toolbar of quick actions. A trigger is a half circle
 * on the left with a lightning mark; logic nodes label their outputs inside the tile; an agent node
 * is a wide tile with configuration blocks under it.
 */
export function CanvasNode({
  node,
  selected = false,
  live = false,
  editable = true,
  tabbable = true,
  focused = false,
  forceToolbar = false,
  detail = 'full',
  labelScale = 1,
  dangling = [],
  invalidInputs = {},
  renaming = false,
  labelled = false,
  actions = {},
  className,
}: CanvasNodeProps) {
  const intl = useIntl();
  const [hovered, setHovered] = useState(false);
  const size = nodeSize(node);
  const trigger = node.kind === 'trigger';
  const placeholder = node.kind === 'placeholder';
  const visual = nodeVisual(node.status);
  const disabled = Boolean(node.disabled) || placeholder;
  const full = detail === 'full';
  const showToolbar = editable && full && (forceToolbar || hovered || selected || focused);

  const inputs = trigger ? [] : portOffsets(node.inputs.length, size.h);
  const outputs = portOffsets(node.outputs.length, size.h);
  const labelledOutputs = node.outputs.length > 1 || node.outputs.some((port) => port.label);

  // How the tile is drawn: the execution state first, then disabled, then a compare mark.
  const family = node.status ? familyClasses[familyOf(node.status)] : undefined;
  const diff = node.diff ? familyClasses[diffFamily[node.diff]] : undefined;
  // Exactly one border color and one surface per state: color classes do not merge, so they must not stack.
  const border = placeholder
    ? 'border border-dashed border-status-warning'
    : node.diff
      ? cn(diff?.border2, node.diff === 'changed' && 'border-dashed', node.diff === 'removed' && 'border-dotted')
      : disabled
        ? 'border border-dashed border-canvas-edge'
        : visual === 'running'
          ? 'border border-status-active'
          : visual === 'failed'
            ? family?.border2
            : visual === 'warning' || visual === 'uncertain' || visual === 'waiting'
              ? family?.border2
              : 'border border-canvas-node-border group-hover/node:border-canvas-edge';
  const surface = placeholder || disabled ? 'bg-surface-sunken' : visual === 'failed' && !node.diff ? 'bg-status-danger-bg' : 'bg-canvas-node-bg';
  const quiet = disabled || visual === 'not-run' || node.diff === 'removed';

  // Chips under the name: the state first, then a mark of the node. Succeeded has none (the quiet state).
  const chips: NodeStatusChipProps[] = [];
  if (placeholder) chips.push({ kind: 'not-supported' });
  else if (node.diff) chips.push({ kind: node.diff });
  else if (disabled) chips.push({ kind: 'disabled' });
  else if (node.status && node.status !== 'SUCCEEDED') chips.push({ status: node.status, code: node.statusCode });
  if ((node.uncertainCount ?? 0) > 0 && node.status !== 'UNCERTAIN') chips.push({ kind: 'uncertain-items', count: node.uncertainCount });
  if (node.pinned) chips.push({ kind: 'pinned' });
  const shownChips = chips.slice(0, 2);

  const ringRadius = trigger
    ? `var(--orcon-radius-full) calc(var(--orcon-radius-2xl) + var(--orcon-canvas-ring-gap)) calc(var(--orcon-radius-2xl) + var(--orcon-canvas-ring-gap)) var(--orcon-radius-full)`
    : 'calc(var(--orcon-radius-2xl) + var(--orcon-canvas-ring-gap))';

  const tileStyle: CSSProperties | undefined = size.h === metrics.tile ? tileRadiusStyle(node) : { ...tileRadiusStyle(node), height: size.h };
  const labelStyle: CSSProperties | undefined =
    labelScale === 1 ? undefined : { transform: `translateX(-50%) scale(${labelScale})`, transformOrigin: 'top center' };
  const aiBlocks = node.ai ?? [];

  const toolbar = showToolbar ? (
    <NodeToolbar
      name={node.name}
      trigger={trigger}
      disabled={disabled}
      pinned={node.pinned}
      canRun={!placeholder}
      tabbable={tabbable}
      onExecuteWorkflow={() => actions.executeWorkflow?.(node.id)}
      onRunStep={() => actions.runStep?.(node.id)}
      onToggleDisabled={() => actions.toggleDisabled?.(node.id)}
      onDelete={() => actions.remove?.(node.id)}
      onRename={() => actions.startRename?.(node.id)}
      onDuplicate={() => actions.duplicate?.(node.id)}
      onTogglePin={() => actions.togglePin?.(node.id)}
      onCopy={() => actions.copy?.(node.id)}
      onOpenDetails={() => actions.openDetails?.(node.id)}
    />
  ) : null;

  return (
    <div
      role={labelled ? 'group' : undefined}
      aria-label={labelled ? nodeAriaLabel(intl, node) : undefined}
      title={node.name}
      tabIndex={labelled && !live ? 0 : undefined}
      data-canvas-node={node.id}
      data-node-kind={node.kind}
      data-node-state={disabled ? 'disabled' : visual}
      data-selected={selected || undefined}
      onPointerEnter={() => setHovered(true)}
      onPointerLeave={() => setHovered(false)}
      // The tile height depends on its ports (max(100, 26 x outputs + 22)); the trigger has its own radius.
      // eslint-disable-next-line orcon/no-inline-style -- node geometry computed from its ports
      style={tileStyle}
      className={cn(
        'group/node text-fg relative h-canvas-node',
        node.kind === 'agent' ? 'w-canvas-node-wide' : 'w-canvas-node',
        tileRadius(node),
        className,
      )}
    >
      {toolbar}

      {/* The tile itself: its border, fill and selection outline. Everything else is positioned from the outer edge of the tile. */}
      <span
        aria-hidden
        data-tile-surface
        // The trigger has its own radius; the tile height follows the root.
        // eslint-disable-next-line orcon/no-inline-style -- trigger radius is two different corner radii
        style={tileRadiusStyle(node)}
        className={cn(
          'absolute inset-0 shadow-sm transition-colors',
          tileRadius(node),
          surface,
          border,
          selected && 'outline-canvas-selection outline-2 outline-offset-4',
        )}
      />

      {visual === 'running' && !disabled ? (
        <span
          aria-hidden
          data-run-ring
          // The ring radius is the tile radius plus the gap, so the ring stays concentric with the tile.
          // eslint-disable-next-line orcon/no-inline-style -- ring radius derived from the tile radius
          style={{ borderRadius: ringRadius }}
          className="orcon-run-ring motion-safe:animate-run-ring pointer-events-none absolute -inset-canvas-ring-gap"
        />
      ) : null}

      <span className="absolute inset-0 grid place-items-center" aria-hidden>
        <span className={cn('grid size-canvas-icon-box place-items-center', quiet && 'opacity-60')}>
          <NodeGlyph node={node} muted={quiet} />
        </span>
      </span>

      {labelledOutputs && full
        ? node.outputs.map((port, index) => {
            const text = portLabel(node.outputs, port);
            return text ? (
              <span
                key={port.id}
                aria-hidden
                data-output-label={port.id}
                // eslint-disable-next-line orcon/no-inline-style -- label offset computed from the port position
                style={{ top: outputs[index] }}
                className="text-fg-muted pointer-events-none absolute right-3 -translate-y-1/2 font-mono text-xs leading-4 font-semibold"
              >
                {text}
              </span>
            ) : null;
          })
        : null}

      {trigger ? (
        <span
          role="img"
          aria-label={intl.formatMessage(canvasMessages.triggerMark)}
          data-trigger-mark
          className="bg-canvas-node-bg border-fg-accent text-fg-accent absolute top-1/2 left-0 z-10 grid size-canvas-badge -translate-x-1/2 -translate-y-1/2 place-items-center rounded-full border-2"
        >
          <Zap aria-hidden className="size-3" />
        </span>
      ) : null}

      {node.status && node.status !== 'NOT_RUN' && !disabled ? <NodeStatusBadge status={node.status} className={detail === 'full' ? undefined : 'scale-140'} /> : null}
      {node.pinned ? <PinnedBadge className={detail === 'full' ? undefined : 'scale-140'} /> : null}

      {inputs.map((top, index) => {
        const port = node.inputs[index];
        return port ? (
          <NodePort key={port.id} side="input" id={port.id} top={top} live={live} connectable={editable} invalidReason={invalidInputs[port.id]} />
        ) : null;
      })}
      {outputs.map((top, index) => {
        const port = node.outputs[index];
        if (!port) return null;
        const label = intl.formatMessage(canvasMessages.nodeAddAfter, {
          name: node.name,
          hasOutput: portLabel(node.outputs, port) ? 'yes' : 'no',
          output: portLabel(node.outputs, port) ?? '',
        });
        return (
          <NodePort
            key={port.id}
            side="output"
            id={port.id}
            top={top}
            live={live}
            connectable={editable}
            label={label}
            tabbable={editable && tabbable}
            selected={selected}
            onAdd={() => actions.addAfter?.(node.id, port.id)}
          />
        );
      })}
      {editable && full && !placeholder
        ? outputs.map((top, index) => {
            const port = node.outputs[index];
            if (!port || !dangling.includes(port.id)) return null;
            const label = intl.formatMessage(canvasMessages.nodeAddAfter, {
              name: node.name,
              hasOutput: portLabel(node.outputs, port) ? 'yes' : 'no',
              output: portLabel(node.outputs, port) ?? '',
            });
            return <OutputStub key={`stub-${port.id}`} top={top} label={label} tabbable={tabbable} onAdd={() => actions.addAfter?.(node.id, port.id)} />;
          })
        : null}

      {detail !== 'minimal' ? (
        <div
          // The label scales around its top center so its on-screen size stays readable at low zoom.
          // eslint-disable-next-line orcon/no-inline-style -- counter-scale computed from the canvas zoom
          style={labelStyle}
          data-node-label
          className={cn(
            'pointer-events-none absolute top-full left-1/2 mt-canvas-label-gap flex w-canvas-label-width flex-col items-center text-center',
            labelScale === 1 && '-translate-x-1/2',
            // Under an agent the dashed lines of its blocks pass behind the label.
            node.kind === 'agent' && aiBlocks.length > 0 && 'bg-canvas z-10 rounded-md',
          )}
        >
          {renaming ? (
            <RenameField
              value={node.name}
              label={intl.formatMessage(canvasMessages.renameLabel)}
              onCommit={(name) => actions.commitRename?.(node.id, name)}
              onCancel={() => actions.cancelRename?.(node.id)}
            />
          ) : (
            <span
              data-node-name
              onDoubleClick={(event) => {
                if (!editable) return;
                event.stopPropagation();
                actions.startRename?.(node.id);
              }}
              className="text-fg pointer-events-auto line-clamp-2 text-sm font-semibold wrap-anywhere"
            >
              {node.name}
            </span>
          )}
          {node.subtitle && full ? (
            <span data-node-subtitle className="text-fg-muted w-full truncate text-xs leading-4" data-truncate>
              {node.subtitle}
            </span>
          ) : null}
          {shownChips.length > 0 ? (
            <span className="mt-1 flex flex-col items-center gap-1" data-node-chips>
              {shownChips.map((chip) => (
                <NodeStatusChip key={chip.kind ?? chip.status} {...chip} />
              ))}
            </span>
          ) : null}
        </div>
      ) : null}

      {node.kind === 'agent' && aiBlocks.length > 0 && full ? (
        <div
          data-ai-blocks
          className="absolute left-1/2 flex -translate-x-1/2"
          // The blocks hang below the label: a fixed offset from the bottom of the tile.
          // eslint-disable-next-line orcon/no-inline-style -- agent block offset from the tile
          style={{ top: size.h }}
        >
          {aiBlocks.map((block) => (
            <div key={block.kind} className="flex w-16 flex-col items-center">
              <span
                aria-hidden
                className="border-canvas-edge border-s-2 border-dashed"
                // eslint-disable-next-line orcon/no-inline-style -- connector line spans the label area under the tile
                style={{ height: AI_BLOCK_OFFSET }}
              />
              <AiBlock kind={block.kind} set={block.set} detail={block.detail} tabbable={editable && tabbable} interactive={editable} onActivate={() => actions.openAiBlock?.(node.id, block.kind)} />
            </div>
          ))}
        </div>
      ) : null}
    </div>
  );
}

interface RenameFieldProps {
  value: string;
  label: string;
  onCommit: (name: string) => void;
  onCancel: () => void;
}

/** The inline field that renames a node under its tile. Enter or leaving the field saves; Escape cancels. */
function RenameField({ value, label, onCommit, onCancel }: RenameFieldProps): ReactNode {
  const [draft, setDraft] = useState(value);
  const field = useRef<HTMLInputElement>(null);
  const done = useRef(false);
  useEffect(() => {
    field.current?.focus();
    field.current?.select();
  }, []);
  const finish = (commit: boolean) => {
    if (done.current) return;
    done.current = true;
    if (commit && draft.trim() !== '') onCommit(draft.trim());
    else onCancel();
  };
  const onKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    event.stopPropagation();
    if (event.key === 'Enter') {
      event.preventDefault();
      finish(true);
    } else if (event.key === 'Escape') {
      event.preventDefault();
      finish(false);
    }
  };
  return (
    <input
      ref={field}
      value={draft}
      aria-label={label}
      onChange={(event) => setDraft(event.target.value)}
      onBlur={() => finish(true)}
      onKeyDown={onKeyDown}
      className="nodrag nopan bg-surface border-control text-fg pointer-events-auto w-full rounded-md border px-1 text-center text-sm font-semibold outline-hidden"
    />
  );
}
