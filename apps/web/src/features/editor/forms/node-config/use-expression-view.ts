// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { useCallback, useState } from 'react';
import { z } from 'zod';

import { readStored, writeStored } from '../../../../app/boundary/storage';

const KEY = 'orcon.editor.expression-view';
const viewSchema = z.enum(['chips', 'raw']);

export type ExpressionView = z.infer<typeof viewSchema>;

/**
 * How formula fields are shown: as chips or as raw text (SPEC-019 R-UX-NDV-03). One click on a field changes that field
 * and is remembered for the person, so the next field, and the next time the editor opens, shows the same way.
 * Chips are the default.
 */
export function useExpressionView(): [ExpressionView, (next: ExpressionView) => void] {
  const [view, setView] = useState<ExpressionView>(() => readStored(KEY, viewSchema, 'chips'));
  const choose = useCallback((next: ExpressionView) => {
    setView(next);
    writeStored(KEY, next);
  }, []);
  return [view, choose];
}
