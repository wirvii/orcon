// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { ConnectorIcon } from '@orcon/ui';

import type { ServiceIcons } from '../model/service-icons';
import { iconSources } from '../model/service-icons';

export interface ServiceIconProps {
  /** The connector reference of the service, the key of its icon. */
  connector: string;
  /** The name of the service: the text alternative of the icon. */
  name: string;
  icons: ServiceIcons | undefined;
}

/**
 * The brand icon of a service. It comes from the catalog of node types; a service whose connector ships no icon
 * (or while the catalog loads) shows the plug glyph, never an empty box or a made-up logo.
 */
export function ServiceIcon({ connector, name, icons }: ServiceIconProps) {
  const sources = iconSources(icons, connector);
  return <ConnectorIcon name={name} src={sources.src} darkSrc={sources.darkSrc} />;
}
