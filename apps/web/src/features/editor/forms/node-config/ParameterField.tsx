// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { FIELD_DRAG_TYPE, FormField, Switch } from '@orcon/ui';
import { Braces } from 'lucide-react';
import { useRef, useState, type DragEvent } from 'react';
import { useIntl } from 'react-intl';

import { carriesField, readDraggedField } from '../../../../app/boundary/drag';
import { isJsonText } from '../../../../app/boundary/parse';
import { useNoticeChannel } from '../../components/notice-channel';
import { appendField, pathLabel } from '../../model/expression-chips';
import { expressionCapable, modeOf, normalize, problemOf, switchMode, valueOf, type ParamMode, type Parameters, type ProblemCode } from '../../model/node-config';
import type { ParamDef } from '../../model/node-manifests';

import { CollectionField } from './CollectionField';
import { ExpressionChipsField } from './ExpressionChipsField';
import { blockFor, useFieldInsert } from './field-insert';
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
import { useExpressionView } from './use-expression-view';
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
  const insert = useFieldInsert();
  const notices = useNoticeChannel();
  const [view, setView] = useExpressionView();
  const label = t(`param.${def.name}.label`);
  const description = optionalText(t, `param.${def.name}.description`);
  const mode = modeOf(def, parameters);
  const value = valueOf(def, parameters);
  const problem = touched ? problemOf(def, parameters, isJsonText) : undefined;
  const write = (next: unknown) => {
    onChange(def.name, normalize(def, next), fieldGroup.group());
  };
  // A field from the input becomes part of the formula, whatever the field showed before: a fixed text turns into a
  // formula that keeps its text. The latest closure is read when it is used, long after the focus that registered it.
  const add = useRef<(block: string) => void>(() => undefined);
  add.current = (block) => {
    const current = parameters[def.name];
    onChange(def.name, appendField(typeof current === 'string' ? current : '', block), fieldGroup.group());
  };
  const register = () => {
    insert.setTarget({ label, apply: (block) => {
        add.current(block);
      } });
  };
  const canTake = expressionCapable(def);
  const onDragOver = (event: DragEvent<HTMLDivElement>) => {
    if (!canTake || !carriesField(event.dataTransfer, FIELD_DRAG_TYPE)) return;
    event.preventDefault();
    event.dataTransfer.dropEffect = 'copy';
  };
  const onDrop = (event: DragEvent<HTMLDivElement>) => {
    if (!canTake || !carriesField(event.dataTransfer, FIELD_DRAG_TYPE)) return;
    event.preventDefault();
    const path = readDraggedField(event.dataTransfer, FIELD_DRAG_TYPE);
    if (!path) return;
    add.current(blockFor(path, insert.source));
    notices.notify('info', intl.formatMessage(configMessages.chipsDropped, { name: pathLabel(path), field: label }));
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
    <div className="absolute -top-1 end-0 flex items-center gap-1">
      {mode === 'expression' ? (
        <button
          type="button"
          aria-label={intl.formatMessage(view === 'chips' ? configMessages.viewAsText : configMessages.viewAsChips)}
          aria-pressed={view === 'raw'}
          onClick={() => {
            setView(view === 'chips' ? 'raw' : 'chips');
          }}
          className="text-fg hover:bg-surface-sunken aria-pressed:bg-surface-sunken border-control inline-flex min-h-target min-w-target items-center justify-center rounded-md border"
        >
          <Braces aria-hidden className="size-icon-16" />
        </button>
      ) : null}
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
    <div className="relative" onFocusCapture={canTake ? register : undefined} onDragOver={onDragOver} onDrop={onDrop}>
      {modeSwitch}
      <FormField
      label={label}
      description={description}
      required={def.required === true}
      group={group}
      name={`/parameters/${def.name}`}
      error={problem ? intl.formatMessage(problemMessages[problem]) : undefined}
    >
      {mode === 'expression' && expressionCapable(def) ? (
        view === 'chips' ? (
          <ExpressionChipsField
            value={parameters[def.name]}
            label={label}
            onFocus={fieldGroup.renew}
            onBlur={() => {
              setTouched(true);
            }}
            onChange={(stored) => {
              onChange(def.name, stored, fieldGroup.group());
            }}
          />
        ) : (
          <ExpressionControl {...props} value={parameters[def.name]} />
        )
      ) : (
        <Widget {...props} />
      )}
      </FormField>
    </div>
  );
}
