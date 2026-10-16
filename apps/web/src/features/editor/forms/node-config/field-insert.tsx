// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createContext, useContext, useLayoutEffect, useMemo, useRef, type ReactNode } from 'react';

import { fieldExpression, type FieldRef } from '../../model/expression-chips';

/** The step whose data the input pane shows, and whether it is the only step that feeds this one. */
export interface InputSource {
  /** The name of the step the fields come from. */
  readonly node: string;
  /** The only step that feeds this one: its fields are written `$json.a`, any other `$('Step').item.json.a`. */
  readonly direct: boolean;
}

/** The parameter that last had the focus: the one a field chosen with the keyboard goes to. */
export interface InsertTarget {
  /** The label of the parameter, for the confirmation. */
  readonly label: string;
  /** Adds the block to the parameter. */
  readonly apply: (block: string) => void;
}

interface FieldInsert {
  readonly source: InputSource | undefined;
  /** Registers the parameter that has the focus. */
  readonly setTarget: (target: InsertTarget | undefined) => void;
}

const idle: FieldInsert = { source: undefined, setTarget: () => undefined };

const Context = createContext<FieldInsert>(idle);

/** What a parameter needs to take a field from the input pane: where fields come from and a way to say it has the focus. */
export const useFieldInsert = (): FieldInsert => useContext(Context);

/** The block a field of the input makes for a parameter: `{{ $json.a }}` from the only step that feeds this one, a reference to the step otherwise. */
export function blockFor(path: FieldRef['path'], source: InputSource | undefined): string {
  return fieldExpression({ node: source?.node ?? '', path }, source?.direct ?? true);
}

export interface FieldInsertController {
  /** Adds a field chosen in the input pane to the parameter that had the focus; says which, or `undefined` when none had it. */
  insert: (path: FieldRef['path']) => string | undefined;
}

/**
 * Provides the input source to the parameters and keeps the parameter that last had the focus, so Enter on a field of
 * the input pane (SPEC-014 R-ED-EXPR-01) writes it there. `controller` is how the input pane reaches it.
 */
export function FieldInsertProvider({ source, controller, children }: { source: InputSource | undefined; controller: { current: FieldInsertController | undefined }; children: ReactNode }) {
  const target = useRef<InsertTarget | undefined>(undefined);
  const value = useMemo<FieldInsert>(
    () => ({
      source,
      setTarget: (next) => {
        target.current = next;
      },
    }),
    [source],
  );
  useLayoutEffect(() => {
    controller.current = {
      insert: (path) => {
        const found = target.current;
        if (!found) return undefined;
        found.apply(blockFor(path, source));
        return found.label;
      },
    };
    return () => {
      controller.current = undefined;
    };
  }, [controller, source]);
  return <Context.Provider value={value}>{children}</Context.Provider>;
}
