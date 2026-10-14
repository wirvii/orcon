// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { FlowField } from '../forms/FlowField';

// Only the editor's components may import forms: this one may not.
export function FlowPanel() {
  return <FlowField />;
}
