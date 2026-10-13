// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { MiniMap } from '@xyflow/react';
import { useIntl } from 'react-intl';

import { cn } from '../../lib/cn';
import { canvasMessages } from '../messages';
import { metrics } from '../model';

export interface CanvasMinimapProps {
  /** Shown while panning or zooming and for a second after, and while hovered. */
  visible?: boolean | undefined;
  /** Pointer enter and leave keep it open while the pointer is on it. */
  onPointerEnter?: (() => void) | undefined;
  onPointerLeave?: (() => void) | undefined;
  className?: string | undefined;
}

/**
 * The overview map at the bottom right: nodes as small rectangles, the selected ones in the selection
 * color, and the viewport as a rectangle with a 2 px selection border. Clicking or dragging it moves
 * the viewport. It fades out when the canvas is at rest. It must be rendered inside the canvas.
 */
export function CanvasMinimap({ visible = true, onPointerEnter, onPointerLeave, className }: CanvasMinimapProps) {
  const intl = useIntl();
  return (
    <MiniMap
      pannable
      zoomable
      position="bottom-right"
      // The map has the size of the minimap tokens, which only the library can apply.
      // eslint-disable-next-line orcon/no-inline-style -- minimap size comes from the canvas tokens
      style={{ width: metrics.minimapW, height: metrics.minimapH }}
      ariaLabel={intl.formatMessage(canvasMessages.ariaMinimap)}
      nodeColor={(node) => (node.selected ? 'var(--orcon-canvas-selection)' : 'var(--orcon-canvas-node-border)')}
      nodeStrokeColor="var(--orcon-canvas-node-border)"
      bgColor="var(--orcon-surface-raised)"
      maskColor="var(--orcon-surface-sunken)"
      maskStrokeColor="var(--orcon-canvas-selection)"
      maskStrokeWidth={2}
      nodeBorderRadius={4}
      onPointerEnter={onPointerEnter}
      onPointerLeave={onPointerLeave}
      data-canvas-minimap
      className={cn(
        'border-border m-0 mb-8 overflow-hidden rounded-md border shadow-md transition-opacity',
        visible ? 'opacity-100' : 'pointer-events-none opacity-0',
        className,
      )}
    />
  );
}
