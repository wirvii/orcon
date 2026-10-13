// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

export interface Execution {
  readonly id: string;
  readonly status: 'RUNNING' | 'FAILED';
}
