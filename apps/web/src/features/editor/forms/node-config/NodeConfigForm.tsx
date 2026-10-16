// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Banner, FormField, Textarea } from '@orcon/ui';
import { useState } from 'react';
import { useIntl, type MessageDescriptor } from 'react-intl';

import { parseJsonText } from '../../../../app/boundary/parse';
import { pinOf } from '../../model/connector-versions';
import type { FlowNode } from '../../model/flow-document';
import { credentialSlotsOf, slotInManifest } from '../../model/credential-slots';
import { credentialAt } from '../../model/document-ops';
import { hiddenWithValue, isVisible } from '../../model/node-config';
import { isUnsupported, manifestOf, parametersFor, type NodeManifest } from '../../model/node-manifests';

import { CredentialSelect } from './CredentialSelect';
import { FlowTargetField } from './FlowTargetField';
import { configMessages } from './messages';
import { ParameterField } from './ParameterField';
import { useCatalog, useFieldGroup, useNode, useNodeActions } from './use-node-config';

export interface NodeConfigFormProps {
  nodeId: string;
  /** The schema to render. Defaults to the manifest of the node's type; tests and the API view pass their own. */
  manifest?: NodeManifest | undefined;
}

const SLOT_TEXT: Partial<Record<string, { label: MessageDescriptor; note: MessageDescriptor }>> = {
  whatsapp: { label: configMessages.slotWhatsappLabel, note: configMessages.slotWhatsappNote },
  model: { label: configMessages.slotModelLabel, note: configMessages.slotModelNote },
};

/** The parameters of a step type the editor has no schema for: the raw object, edited as JSON. */
function RawParameters({ node }: { node: FlowNode }) {
  const intl = useIntl();
  const actions = useNodeActions(node.id);
  const group = useFieldGroup('raw-parameters');
  const [text, setText] = useState(() => JSON.stringify(node.parameters, null, 2));
  const [invalid, setInvalid] = useState(false);
  return (
    <div className="flex flex-col gap-3">
      <Banner tone="info" title={intl.formatMessage(configMessages.unknownTitle)}>
        {intl.formatMessage(configMessages.unknownText)}
      </Banner>
      <FormField label={intl.formatMessage(configMessages.unknownLabel)} name="/parameters" error={invalid ? intl.formatMessage(configMessages.notJson) : undefined}>
        <Textarea
          rows={10}
          className="font-mono text-mono"
          spellCheck={false}
          value={text}
          onFocus={group.renew}
          onChange={(event) => {
            const next = event.target.value;
            setText(next);
            const parsed = parseJsonText(next);
            if (!parsed.ok || typeof parsed.value !== 'object' || parsed.value === null || Array.isArray(parsed.value)) {
              setInvalid(true);
              return;
            }
            setInvalid(false);
            const object = parsed.value as Record<string, unknown>;
            for (const [name, value] of Object.entries(object)) actions.setParameter(name, value, group.group());
            for (const name of Object.keys(node.parameters)) if (!(name in object)) actions.setParameter(name, undefined, group.group());
          }}
        />
      </FormField>
    </div>
  );
}

/**
 * The one renderer of node configuration (SPEC-010 FM-7, SPEC-014 R-ED-CFG-01): it draws the
 * credential slots and the parameters of a node from the schema of its type, one widget per
 * parameter type, hides what `displayWhen` hides and what the pinned connector version does not
 * have, and writes every change to the draft through the store. There is no hand-written form per
 * node type. A type without a schema falls back to the raw parameters as JSON.
 */
export function NodeConfigForm({ nodeId, manifest: override }: NodeConfigFormProps) {
  const intl = useIntl();
  const node = useNode(nodeId);
  const actions = useNodeActions(nodeId);
  const manifest = override ?? (node ? manifestOf(node) : undefined);
  const t = useCatalog(manifest);
  if (!node) return null;
  if (isUnsupported(node)) {
    return (
      <Banner tone="warning" title={intl.formatMessage(configMessages.unsupportedTitle)}>
        {intl.formatMessage(configMessages.unsupportedText)}
      </Banner>
    );
  }
  const slots = credentialSlotsOf(node);
  const slotTexts = (slot: (typeof slots)[number]) => {
    const own = SLOT_TEXT[slot.slot];
    return slotInManifest(node, slot) || own === undefined ? { label: t(`credential.${slot.slot}.label`), note: t(`credential.${slot.slot}.note`) } : { label: intl.formatMessage(own.label), note: intl.formatMessage(own.note) };
  };
  if (!manifest) {
    // A step type the editor has no schema for still takes its credential from the list; the rest of its parameters are the raw object.
    // The raw text restarts when the credential changes, because the credential is one of those parameters.
    const held = slots.map((slot) => credentialAt(node, slot.parameter) ?? '').join('|');
    return (
      <div className="flex flex-col gap-4" data-node-config="raw">
        {slots.map((slot) => (
          <CredentialSelect key={slot.slot} nodeId={nodeId} slot={slot} bound={credentialAt(node, slot.parameter)} {...slotTexts(slot)} />
        ))}
        <RawParameters key={held} node={node} />
      </div>
    );
  }

  const known = parametersFor(manifest, pinOf(node)?.version);
  const defs = known.filter((def) => isVisible(def, node.parameters, known));
  const kept = hiddenWithValue(known, node.parameters);
  if (defs.length === 0 && slots.length === 0) {
    return <p className="text-fg-muted text-sm">{intl.formatMessage(configMessages.noParameters)}</p>;
  }
  return (
    <div className="flex flex-col gap-4" data-node-config={manifest.key}>
      {slots.map((slot) => (
        <CredentialSelect key={slot.slot} nodeId={nodeId} slot={slot} bound={credentialAt(node, slot.parameter)} {...slotTexts(slot)} />
      ))}
      {defs.map((def) =>
        def.widget === 'flow-target' ? (
          <FlowTargetField key={def.name} nodeId={nodeId} value={node['call']} />
        ) : (
          <ParameterField key={def.name} def={def} parameters={node.parameters} t={t} onChange={actions.setParameter} />
        ),
      )}
      {kept.length > 0 ? (
        <p className="text-fg-muted text-sm" data-hidden-values>
          {intl.formatMessage(configMessages.hiddenValues, { count: kept.length, names: kept.map((def) => t(`param.${def.name}.label`)).join(', ') })}
        </p>
      ) : null}
    </div>
  );
}
