// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { useIntl } from 'react-intl';

import type { FlowNode } from '../../model/flow-document';
import { catalogText, localeOf, safetyOf, type NodeManifest } from '../../model/node-manifests';

import { DuplicateProtectionBox } from './DuplicateProtectionBox';
import { configMessages } from './messages';
import { NodeSettingsForm } from './NodeSettingsForm';

/**
 * The Retries and safety tab (SPEC-019 R-UX-NDV-06): the box about sending twice, with the unique key when the step has
 * one, and the retry settings of the step (SPEC-001) under it. The retry settings are the ones the Settings tab used to
 * hold: they moved here, next to what they protect.
 */
export function RetrySafetyTab({ node, manifest }: { node: FlowNode; manifest: NodeManifest | undefined }) {
  const intl = useIntl();
  const safety = safetyOf(node);
  const service = manifest ? catalogText(manifest, localeOf(intl.locale), 'name') : node.name;
  return (
    <div className="flex flex-col gap-4" data-retry-safety>
      <p className="text-fg-muted text-sm">{intl.formatMessage(configMessages.retryIntro)}</p>
      <DuplicateProtectionBox safety={safety} service={service} />
      {safety === undefined ? <p className="text-fg-muted text-sm">{intl.formatMessage(configMessages.retryNothingOutside)}</p> : null}
      <NodeSettingsForm node={node} manifest={manifest} section="retry" />
    </div>
  );
}
