// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { FormField, Switch } from '@orcon/ui';
import { useState } from 'react';
import { useIntl } from 'react-intl';

import { isJsonText } from '../../../../app/boundary/parse';
import { expressionCapable, modeOf, normalize, problemOf, switchMode, valueOf, type ParamMode, type Parameters, type ProblemCode } from '../../model/node-config';
import type { ParamDef } from '../../model/node-manifests';

import { CollectionField } from './CollectionField';
import {
  BinaryField,
  DateTimeField,
  ExpressionControl,
  JsonField,
  MultiOptionsField,
  NumberField,
  OptionsField,
  ResourceLocatorField,
  StringField,
  type FieldProps,
} from './fields';
import { configMessages } from './messages';
import { useFieldGroup } from './use-node-config';

const problemMessages = {
  required: configMessages.required,
  number: configMessages.notNumber,
  json: configMessages.notJson,
  option: configMessages.unknownOption,
  incomplete: configMessages.incompleteEntry,
} as const satisfies Record<ProblemCode, unknown>;

/** The fixed-value widget of each parameter type (SPEC-014 R-ED-CFG-01: type to widget). */
const widgets: Record<Exclude<ParamDef['type'], 'BOOLEAN'>, (props: FieldProps) => React.JSX.Element> = {
  STRING: StringField,
  NUMBER: NumberField,
  OPTIONS: OptionsField,
  MULTI_OPTIONS: MultiOptionsField,
  COLLECTION: CollectionField,
  FIXED_COLLECTION: CollectionField,
  JSON: JsonField,
  DATE_TIME: DateTimeField,
  RESOURCE_LOCATOR: ResourceLocatorField,
  BINARY_PROPERTY: BinaryField,
};

const MODES = ['fixed', 'expression'] as const;

/** The Fixed / Expression switch of a text-like field, at the right of its label. */
function ModeSwitch({ label, mode, onChange }: { label: string; mode: ParamMode; onChange: (mode: ParamMode) => void }) {
  const intl = useIntl();
  return (
    <div role="group" aria-label={intl.formatMessage(configMessages.valueKind, { field: label })} className="border-control inline-flex shrink-0 overflow-hidden rounded-md border">
      {MODES.map((kind) => (
        <button
          key={kind}
          type="button"
          aria-pressed={mode === kind}
          onClick={() => {
            if (mode !== kind) onChange(kind);
          }}
          className="text-fg bg-surface hover:bg-surface-sunken aria-pressed:bg-surface-sunken min-h-target px-2 text-xs aria-pressed:font-semibold"
        >
          {intl.formatMessage(kind === 'fixed' ? configMessages.fixed : configMessages.expression)}
        </button>
      ))}
    </div>
  );
}

interface ParameterFieldProps {
  def: ParamDef;
  parameters: Parameters;
  t: (key: string) => string;
  onChange: (name: string, value: unknown, group?: string) => void;
}

const optionalText = (t: (key: string) => string, key: string): string | undefined => {
  const found = t(key);
  return found === key ? undefined : found;
};

/**
 * One parameter of a node: its label and description from the catalog, the widget of its type, the
 * Fixed / Expression switch when the type allows a formula, and the problem with its value once the
 * person has left the field (SPEC-014 R-ED-CFG-01, R-ED-CFG-02, R-ED-CFG-06).
 */
export function ParameterField({ def, parameters, t, onChange }: ParameterFieldProps) {
  const intl = useIntl();
  const [touched, setTouched] = useState(false);
  const fieldGroup = useFieldGroup(def.name);
  const label = t(`param.${def.name}.label`);
  const description = optionalText(t, `param.${def.name}.description`);
  const mode = modeOf(def, parameters);
  const value = valueOf(def, parameters);
  const problem = touched ? problemOf(def, parameters, isJsonText) : undefined;
  const write = (next: unknown) => {
    onChange(def.name, normalize(def, next), fieldGroup.group());
  };

  if (def.type === 'BOOLEAN') {
    return (
      <div className="flex flex-col gap-1">
        <Switch
          label={label}
          checked={value === true}
          onCheckedChange={(checked) => {
            onChange(def.name, checked);
          }}
        />
        {description ? <p className="text-fg-muted text-xs">{description}</p> : null}
      </div>
    );
  }

  const props: FieldProps = {
    def,
    value,
    t,
    onChange: write,
    onFocus: fieldGroup.renew,
    onBlur: () => {
      setTouched(true);
    },
  };
  const Widget = widgets[def.type];
  const group = def.type === 'FIXED_COLLECTION' || def.type === 'COLLECTION' || def.type === 'MULTI_OPTIONS';
  const modeSwitch = expressionCapable(def) ? (
    // The switch sits at the right end of the label row, over the space the label leaves free.
    <div className="absolute -top-1 end-0">
      <ModeSwitch
        label={label}
        mode={mode}
        onChange={(next) => {
          onChange(def.name, switchMode(def, parameters, next));
        }}
      />
    </div>
  ) : null;
  return (
    <div className="relative">
      {modeSwitch}
      <FormField
      label={label}
      description={description}
      required={def.required === true}
      group={group}
      name={`/parameters/${def.name}`}
      error={problem ? intl.formatMessage(problemMessages[problem]) : undefined}
    >
      {mode === 'expression' && expressionCapable(def) ? <ExpressionControl {...props} value={parameters[def.name]} /> : <Widget {...props} />}
      </FormField>
    </div>
  );
}
