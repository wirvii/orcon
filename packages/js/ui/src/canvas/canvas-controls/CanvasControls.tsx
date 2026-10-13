// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Maximize, RotateCcw, Wand, ZoomIn, ZoomOut } from 'lucide-react';
import { useIntl } from 'react-intl';

import { IconButton } from '../../components/icon-button/IconButton';
import { Tooltip } from '../../components/tooltip/Tooltip';
import { cn } from '../../lib/cn';
import { canvasMessages } from '../messages';

export interface CanvasControlsProps {
  /** Current zoom, 1 is 100 %. Reset to 100 % shows only when it differs. */
  zoom: number;
  onFit?: () => void;
  onZoomIn?: () => void;
  onZoomOut?: () => void;
  onReset?: () => void;
  /** Lines the nodes up on the grid. Omit it to hide the button. */
  onTidyUp?: () => void;
  className?: string;
}

const control = 'bg-surface-raised border-border shadow-sm';

/**
 * The zoom controls at the bottom left of the canvas: fit the flow, zoom in, zoom out, reset to
 * 100 % (only when the zoom is not 100 %) and tidy up. Each button has a tooltip with its shortcut.
 */
export function CanvasControls({ zoom, onFit, onZoomIn, onZoomOut, onReset, onTidyUp, className }: CanvasControlsProps) {
  const intl = useIntl();
  const reset = Math.abs(zoom - 1) > 0.005;
  const item = (label: string, shortcut: string | undefined, icon: typeof Maximize, onClick?: () => void) => (
    <Tooltip content={shortcut ? `${label} (${shortcut})` : label} side="right">
      <IconButton variant="secondary" icon={icon} label={label} onClick={onClick} className={control} />
    </Tooltip>
  );
  return (
    <div role="group" aria-label={intl.formatMessage(canvasMessages.ariaControls)} data-canvas-controls className={cn('nopan flex flex-col gap-1', className)}>
      {item(intl.formatMessage(canvasMessages.fitView), '1', Maximize, onFit)}
      {item(intl.formatMessage(canvasMessages.zoomIn), '+', ZoomIn, onZoomIn)}
      {item(intl.formatMessage(canvasMessages.zoomOut), '-', ZoomOut, onZoomOut)}
      {reset ? item(intl.formatMessage(canvasMessages.zoomReset), '0', RotateCcw, onReset) : null}
      {onTidyUp ? item(intl.formatMessage(canvasMessages.tidyUp), undefined, Wand, onTidyUp) : null}
    </div>
  );
}
