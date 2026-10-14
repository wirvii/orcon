// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

export interface Counter {
  readonly value: number;
}

export const next = (counter: Counter): Counter => ({ value: counter.value + 1 });
