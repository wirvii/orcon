// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Plug } from 'lucide-react';
import { useState } from 'react';

import { cn } from '../../lib/cn';
import { useResolvedTheme } from '../../providers/ThemeProvider';

export interface ConnectorIconProps {
  /** URL of the connector's square SVG logo. Without it (or when it does not load) the Plug glyph shows in its place. */
  src?: string | undefined;
  /**
   * URL of the variant of the logo the brand itself publishes for dark backgrounds (usually the same mark in
   * white). Shown in the dark theme. Never a recolored copy: a connector without a published variant has none.
   */
  darkSrc?: string | undefined;
  /** Connector name. It is the image text alternative. */
  name: string;
  /**
   * `frame` (default) is the Orcon frame with the logo in its safe area. `tile` is the bare image for the
   * icon box of a canvas node, which draws its own tile around it.
   */
  variant?: 'frame' | 'tile' | undefined;
  className?: string | undefined;
}

/**
 * The only way a connector logo reaches the page. The logo is an <img>, never
 * inline SVG, so catalog content cannot run script in the app. The frame is
 * 32 px (comfortable) or 24 px (compact) with the logo fitted to a 20/16 px
 * safe area. Third-party logos only identify the service. A connector with no
 * logo shows the Plug glyph, named after the connector, never an empty box.
 */
export function ConnectorIcon({ src, darkSrc, name, variant = 'frame', className }: ConnectorIconProps) {
  const theme = useResolvedTheme();
  const [failed, setFailed] = useState<string | undefined>(undefined);
  const wanted = theme === 'dark' && darkSrc !== undefined ? darkSrc : src;
  const usable = wanted !== undefined && wanted !== failed ? wanted : undefined;

  if (variant === 'tile') {
    if (usable === undefined) {
      return <Plug role="img" aria-label={name} data-connector-fallback className={cn('size-canvas-icon-box text-fg-muted shrink-0 p-1', className)} />;
    }
    return (
      <img
        src={usable}
        alt={name}
        loading="lazy"
        decoding="async"
        data-connector-tile
        onError={() => {
          setFailed(usable);
        }}
        className={cn('size-canvas-icon-box shrink-0 object-contain', className)}
      />
    );
  }
  return (
    <span
      className={cn(
        'bg-surface-sunken border-border inline-flex size-connector-icon shrink-0 items-center justify-center rounded-md border',
        className,
      )}
    >
      {usable === undefined ? (
        <Plug role="img" aria-label={name} data-connector-fallback className="size-connector-icon-safe text-fg-muted" />
      ) : (
        <img
          src={usable}
          alt={name}
          loading="lazy"
          decoding="async"
          onError={() => {
            setFailed(usable);
          }}
          className="size-connector-icon-safe object-contain"
        />
      )}
    </span>
  );
}
