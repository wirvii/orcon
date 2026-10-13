// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

export type Status = 'RUNNING' | 'FAILED' | 'UNCERTAIN';

export function label(status: Status): string {
  switch (status) {
    case 'RUNNING':
      return 'running';
    case 'FAILED':
      return 'failed';
  }
  return 'other';
}
