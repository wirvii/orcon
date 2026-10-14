// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { useIntl } from 'react-intl';

import { pinOf } from '../../model/connector-versions';
import type { FlowNode } from '../../model/flow-document';
import { parametersFor, type NodeManifest } from '../../model/node-manifests';

import { configMessages } from './messages';
import { useCatalog } from './use-node-config';

/**
 * The Docs tab: what the node type does, what each operation does, how its credentials work and
 * what each parameter means. It reads the description and the operation and credential notes of the
 * node-type view, the same texts the connector package ships (SPEC-014 R-ED-CFG-01); nothing is
 * written for the tab.
 */
export function NodeDocs({ node, manifest }: { node: FlowNode; manifest: NodeManifest | undefined }) {
  const intl = useIntl();
  const t = useCatalog(manifest);
  if (!manifest) {
    return (
      <div className="flex flex-col gap-2">
        <p className="text-fg-muted text-sm">{intl.formatMessage(configMessages.docsNone)}</p>
        <p className="text-fg-muted font-mono text-xs">{intl.formatMessage(configMessages.docsVersionUnpinned, { type: node.type })}</p>
      </div>
    );
  }
  const pin = pinOf(node);
  const parameters = parametersFor(manifest, pin?.version).filter((param) => param.widget === undefined || t(`param.${param.name}.label`) !== `param.${param.name}.label`);
  return (
    <div className="flex flex-col gap-4">
      <section className="flex flex-col gap-1">
        <h3 className="text-base font-semibold">{intl.formatMessage(configMessages.docsDescription)}</h3>
        <p className="text-base">{t('description')}</p>
        <p className="text-fg-muted font-mono text-xs">
          {pin ? intl.formatMessage(configMessages.docsVersion, { type: node.type, version: pin.version }) : intl.formatMessage(configMessages.docsVersionUnpinned, { type: node.type })}
        </p>
      </section>
      <section className="flex flex-col gap-1">
        <h3 className="text-base font-semibold">{intl.formatMessage(configMessages.docsOperations)}</h3>
        <ul className="list-disc ps-5 text-base">
          {manifest.operations.map((key) => (
            <li key={key}>{t(key)}</li>
          ))}
        </ul>
      </section>
      {manifest.credentials.length > 0 ? (
        <section className="flex flex-col gap-1">
          <h3 className="text-base font-semibold">{intl.formatMessage(configMessages.docsCredentials)}</h3>
          {manifest.credentials.map((slot) => (
            <p key={slot.slot} className="text-base">
              <span className="font-medium">{t(`credential.${slot.slot}.label`)}: </span>
              {t(`credential.${slot.slot}.note`)}
            </p>
          ))}
        </section>
      ) : null}
      {parameters.length > 0 ? (
        <section className="flex flex-col gap-2">
          <h3 className="text-base font-semibold">{intl.formatMessage(configMessages.docsParameters)}</h3>
          <dl className="flex flex-col gap-2">
            {parameters.map((param) => (
              <div key={param.name} className="flex flex-col">
                <dt className="text-base font-medium">
                  {t(`param.${param.name}.label`)}
                  {param.required === true ? <span className="text-fg-muted text-xs font-normal"> ({intl.formatMessage(configMessages.docsRequired)})</span> : null}
                </dt>
                <dd className="text-fg-muted text-sm">{t(`param.${param.name}.description`) === `param.${param.name}.description` ? '' : t(`param.${param.name}.description`)}</dd>
              </div>
            ))}
          </dl>
        </section>
      ) : null}
    </div>
  );
}
