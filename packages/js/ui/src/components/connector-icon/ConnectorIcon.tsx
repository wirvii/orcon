// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { cn } from '../../lib/cn';

export interface ConnectorIconProps {
  /** URL of the connector's square SVG logo. */
  src: string;
  /** Connector name. It is the image text alternative. */
  name: string;
  className?: string;
}

/**
 * The only way a connector logo reaches the page. The logo is an <img>, never
 * inline SVG, so catalog content cannot run script in the app. The frame is
 * 32 px (comfortable) or 24 px (compact) with the logo fitted to a 20/16 px
 * safe area. Third-party logos only identify the service.
 */
export function ConnectorIcon({ src, name, className }: ConnectorIconProps) {
  return (
    <span
      className={cn(
        'bg-surface-sunken border-border inline-flex size-connector-icon shrink-0 items-center justify-center rounded-md border',
        className,
      )}
    >
      <img src={src} alt={name} loading="lazy" decoding="async" className="size-connector-icon-safe object-contain" />
    </span>
  );
}
